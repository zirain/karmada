package wasminterpreter

import (
	"context"
	"fmt"
	"io/ioutil"

	jsonpatch "github.com/evanphx/json-patch/v5"
	"github.com/wasmerio/wasmer-go/wasmer"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	webhookutil "k8s.io/apiserver/pkg/util/webhook"
	"k8s.io/klog/v2"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/resourceinterpreter/customizedinterpreter/webhook"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/genericmanager"
)

// WasmInterpreter interpret custom resource with webhook configuration.
type WasmInterpreter struct {
	configManager *webhookutil.ClientManager
}

// NewWasmInterpreter return a new CustomizedInterpreter.
func NewWasmInterpreter(informer genericmanager.SingleClusterInformerManager) (*WasmInterpreter, error) {
	cm, err := webhookutil.NewClientManager(
		[]schema.GroupVersion{configv1alpha1.SchemeGroupVersion},
		configv1alpha1.AddToScheme,
	)
	if err != nil {
		return nil, err
	}
	authInfoResolver, err := webhookutil.NewDefaultAuthenticationInfoResolver("")
	if err != nil {
		return nil, err
	}
	cm.SetAuthenticationInfoResolver(authInfoResolver)
	cm.SetServiceResolver(webhookutil.NewDefaultServiceResolver())

	return &WasmInterpreter{
		configManager: &cm,
	}, nil
}

// HookEnabled tells if any hook exist for specific resource gvk and operation type.
func (e *WasmInterpreter) HookEnabled(objGVK schema.GroupVersionKind, operation configv1alpha1.InterpreterOperation) bool {
	return true
}

// GetReplicas returns the desired replicas of the object as well as the requirements of each replica.
// return matched value to indicate whether there is a matching hook.
func (e *WasmInterpreter) GetReplicas(ctx context.Context, attributes *webhook.RequestAttributes) (replica int32, requires *workv1alpha2.ReplicaRequirements, matched bool, err error) {
	var response *webhook.ResponseAttributes
	response, matched, err = e.interpret(ctx, attributes)
	if err != nil {
		return
	}
	if !matched {
		return
	}

	return response.Replicas, response.ReplicaRequirements, matched, nil
}

// Patch returns the Unstructured object that applied patch response that based on the RequestAttributes.
// return matched value to indicate whether there is a matching hook.
func (e *WasmInterpreter) Patch(ctx context.Context, attributes *webhook.RequestAttributes) (obj *unstructured.Unstructured, matched bool, err error) {
	var response *webhook.ResponseAttributes
	response, matched, err = e.interpret(ctx, attributes)
	if err != nil {
		return
	}
	if !matched {
		return
	}
	obj, err = applyPatch(attributes.Object, response.Patch, response.PatchType)
	if err != nil {
		return
	}
	return
}

func (e *WasmInterpreter) interpret(ctx context.Context, attributes *webhook.RequestAttributes) (*webhook.ResponseAttributes, bool, error) {
	wasmBytes, err := ioutil.ReadFile("/etc/karmada/interpreter.wasm")
	if err != nil {
		klog.Errorf("load wasmer module err: %v", err)
		return nil, false, nil
	}

	engine := wasmer.NewEngine()
	store := wasmer.NewStore(engine)

	// Compiles the module
	module, err := wasmer.NewModule(store, wasmBytes)
	if err != nil {
		klog.Errorf("new wasmer module err: %v", err)
		return nil, false, nil
	}

	wasiEnv, _ := wasmer.NewWasiStateBuilder("wasi-program").Finalize()

	// Instantiates the module
	importObject, err := wasiEnv.GenerateImportObject(store, module)
	if err != nil {
		klog.Errorf("generate wasmer objecer err: %v", err)
		return nil, false, nil
	}
	instance, err := wasmer.NewInstance(module, importObject)
	if err != nil {
		klog.Errorf("new wasmer instance err: %v", err)
		return nil, false, nil
	}

	// Gets the `sum` exported function from the WebAssembly instance.
	fn, err := instance.Exports.GetFunction("Interpreter")
	if err != nil {
		klog.Errorf("get wasmer module err: %v", err)
		return nil, false, nil
	}

	// Calls that exported function with Go standard values. The WebAssembly
	// types are inferred and values are casted automatically.
	result, _ := fn(attributes)
	attrs, ok := result.(*webhook.ResponseAttributes)
	if !ok {
		return nil, false, nil
	}

	klog.V(2).Infof("interpret results: %v", attrs)

	return attrs, false, nil
}

// applyPatch uses patchType mode to patch object.
func applyPatch(object *unstructured.Unstructured, patch []byte, patchType configv1alpha1.PatchType) (*unstructured.Unstructured, error) {
	if len(patch) == 0 && len(patchType) == 0 {
		klog.Infof("Skip apply patch for object(%s: %s) as patch and patchType is nil", object.GroupVersionKind().String(), object.GetName())
		return object, nil
	}
	switch patchType {
	case configv1alpha1.PatchTypeJSONPatch:
		if len(patch) == 0 {
			return object, nil
		}
		patchObj, err := jsonpatch.DecodePatch(patch)
		if err != nil {
			return nil, err
		}
		if len(patchObj) == 0 {
			return object, nil
		}

		objectJSONBytes, err := object.MarshalJSON()
		if err != nil {
			return nil, err
		}
		patchedObjectJSONBytes, err := patchObj.Apply(objectJSONBytes)
		if err != nil {
			return nil, err
		}

		err = object.UnmarshalJSON(patchedObjectJSONBytes)
		return object, err
	default:
		return nil, fmt.Errorf("return patch type %s is not support", patchType)
	}
}

// GetDependencies returns the dependencies of give object.
// return matched value to indicate whether there is a matching hook.
func (e *WasmInterpreter) GetDependencies(ctx context.Context, attributes *webhook.RequestAttributes) (dependencies []configv1alpha1.DependentObjectReference, matched bool, err error) {
	var response *webhook.ResponseAttributes
	response, matched, err = e.interpret(ctx, attributes)
	if err != nil {
		return
	}
	if !matched {
		return
	}

	return response.Dependencies, matched, nil
}

// ReflectStatus returns the status of the object.
// return matched value to indicate whether there is a matching hook.
func (e *WasmInterpreter) ReflectStatus(ctx context.Context, attributes *webhook.RequestAttributes) (status *runtime.RawExtension, matched bool, err error) {
	var response *webhook.ResponseAttributes
	response, matched, err = e.interpret(ctx, attributes)
	if err != nil {
		return
	}
	if !matched {
		return
	}

	return &response.RawStatus, matched, nil
}

// InterpretHealth returns the health state of the object.
// return matched value to indicate whether there is a matching hook.
func (e *WasmInterpreter) InterpretHealth(ctx context.Context, attributes *webhook.RequestAttributes) (healthy bool, matched bool, err error) {
	var response *webhook.ResponseAttributes
	response, matched, err = e.interpret(ctx, attributes)
	if err != nil {
		return
	}
	if !matched {
		return
	}

	return response.Healthy, matched, nil
}
