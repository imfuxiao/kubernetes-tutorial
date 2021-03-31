package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/imfuxiao/kubernetes-tutorial/custom-resource-definetion/k8s-controller-crd-resource/pkg/apis/etcd/v1alpha1"
	clientSetEtcd "github.com/imfuxiao/kubernetes-tutorial/custom-resource-definetion/k8s-controller-crd-resource/pkg/client/clientset/versioned"
	informerEtcdV1Alpha1 "github.com/imfuxiao/kubernetes-tutorial/custom-resource-definetion/k8s-controller-crd-resource/pkg/client/informers/externalversions/etcd/v1alpha1"
	listersEtcdV1Alpha1 "github.com/imfuxiao/kubernetes-tutorial/custom-resource-definetion/k8s-controller-crd-resource/pkg/client/listers/etcd/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/json"
	utilRuntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedCoreV1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	"k8s.io/utils/pointer"
	"time"
)

const (
	controllerAgentName = "etcd-controller"
)

const (
	// SuccessSynced is used as part of the Event 'reason' when a Etcd is synced
	SuccessSynced = "Synced"

	// MessageResourceSynced is the message used for an Event fired when a Etcd
	// is synced successfully
	MessageResourceSynced = "Etcd synced successfully"
)

type Controller struct {
	clientSet     kubernetes.Interface
	etcdClientSet clientSetEtcd.Interface

	etcdLister listersEtcdV1Alpha1.EtcdLister
	etcdSynced cache.InformerSynced

	workerQueue workqueue.RateLimitingInterface
	recorder    record.EventRecorder
}

func NewController(kubeClientSet kubernetes.Interface,
	etcdClientSet clientSetEtcd.Interface,
	etcdInformer informerEtcdV1Alpha1.EtcdInformer) *Controller {

	// 创建事件广播
	// Add sample-controller types to the default Kubernetes Scheme so Events can be logged for sample-controller types.
	// 将样本控制器类型添加到默认的Kubernetes方案，以便可以记录样本控制器类型的事件。
	utilRuntime.Must(v1alpha1.AddToScheme(scheme.Scheme))
	klog.V(4).Info("create event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&typedCoreV1.EventSinkImpl{
		Interface: kubeClientSet.CoreV1().Events(""),
	})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	controller := &Controller{
		clientSet:     kubeClientSet,
		etcdClientSet: etcdClientSet,
		etcdLister:    etcdInformer.Lister(),
		etcdSynced:    etcdInformer.Informer().HasSynced,
		workerQueue:   workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "etcd"),
		recorder:      recorder,
	}

	klog.Info("Setting up event handlers")
	// Set Up an event handler for when etcd resource change
	etcdInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueueEtcd,
		UpdateFunc: func(oldObj, newObj interface{}) {
			// informer resync 会触发 updateFunc, 而此时对象可能没有发生变化, 所以必须要在处理
			oldEtcdObj := oldObj.(*v1alpha1.Etcd)
			newEtcdObj := newObj.(*v1alpha1.Etcd)
			if oldEtcdObj.ResourceVersion == newEtcdObj.ResourceVersion {
				return
			}

			controller.enqueueEtcd(newObj)
		},
		DeleteFunc: controller.enqueueEtcdForDelete,
	})
	return controller
}

func (c *Controller) Run(maxGoroutineCount int, stop <-chan struct{}) error {
	defer utilRuntime.HandleCrash()
	defer c.workerQueue.ShutDown()

	klog.Info("Starting Network control loop")

	klog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stop, c.etcdSynced); !ok {
		return errors.New("failed to wait for caches to sync")
	}

	klog.Info("Starting workers")
	for i := 0; i < maxGoroutineCount; i++ {
		go wait.Until(c.runWorker, time.Second, stop)
	}

	klog.Info("Started workers")
	<-stop
	klog.Info("Shutting down workers")

	return nil
}

func (c *Controller) runWorker() {
	for c.processNextWorkItem() {
	}
}

func (c *Controller) processNextWorkItem() bool {
	obj, shutdown := c.workerQueue.Get()
	if shutdown {
		return false
	}

	err := func(obj interface{}) error {
		// 我们在这里调用Done()，这样工作队列就知道我们已经完成了该项目的处理。
		// 我们还必须记住，如果我们调用Forget()表示不希望此工作项重新排队。
		// 例如，我们如果发生暂时性错误，则不调用Forget，而是将该项目放回工作队列，并在下次运行时继续尝试。
		defer c.workerQueue.Done(obj)

		key, ok := obj.(string)
		if !ok {
			// 由于工作队列中的项目实际上是无效的，我们忘了这里，否则我们将陷入尝试处理无效的工作项。
			c.workerQueue.Forget(obj)
			return fmt.Errorf("expected string int workqueue but got %#v", obj)
		}

		// 运行syncHandler，并向其传递名称空间/名称字符串
		if err := c.syncHandler(key); err != nil {
			klog.Errorf("error syncing:  key =  %s, error: %s", key, err.Error())
			return errors.Unwrap(err)
		}

		// 最后，如果没有发生错误，我们将忽略此项目，因此不会再次排队，直到发生其他更改为止。
		c.workerQueue.Forget(obj)
		klog.Infof("Successfully synced '%s'", key)
		return nil
	}(obj)

	if err != nil {
		utilRuntime.HandleError(err)
	}
	return true
}

// syncHandler将实际状态与所需状态进行比较，并尝试将两者融合。然后，它更新etcd status以及资源的当前状态。
func (c *Controller) syncHandler(key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		utilRuntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	// 访问本地缓存的索引
	// 各种 Lister 里获取对象，比如：podLister、nodeLister 等等，它们使用的都是 Informer 和缓存机制。
	etcd, err := c.etcdLister.Etcds(namespace).Get(name)
	if err != nil {

		// 如果从缓存中拿不到这个对象（即：返回了 IsNotFound 错误
		// 那就意味着这个 etcd 对象的 Key 是通过前面的"删除"事件(DeleteFunc)添加进工作队列的
		// 所以，尽管队列里有这个 Key，但是对应的 etcd 对象已经被删除了。
		if apiErrors.IsNotFound(err) {
			klog.Warningf("etcd: %s/%s does not exists in local cache. will delete it from k8s...",
				namespace, name)
			err = c.etcdClientSet.CrdV1alpha1().Etcds(namespace).Delete(context.Background(),
				name, metav1.DeleteOptions{})
			if err != nil {
				utilRuntime.HandleError(err)
			}
			err = c.clientSet.AppsV1().Deployments(namespace).Delete(context.Background(), name, metav1.DeleteOptions{})
			if err != nil {
				utilRuntime.HandleError(err)
			}
		}
		utilRuntime.HandleError(err)
		return err
	}

	klog.Infof("[Etcd] Try to process etcd: %#v ...", etcd)

	// 执行控制器模式里的对比"期望状态"和"实际状态"的逻辑了。
	// 自定义控制器拿到的这个 etcd 对象，正是 APIServer 里保存的"期望状态"，即：用户通过 YAML 文件提交到 APIServer 里的信息。
	// 当然，在此示例里，它已经被 Informer 缓存在了本地。

	// “实际状态”又从哪里来呢? 从集群中获取. 从集群中获取etcd对应的deployment,
	// 如果deloyment不存在, 则需要创建一个新的
	// 如果存在, 则读取这个deployment信息, 看是否与期望状态信息一致, 如果不一致则需要更新

	// TODO 对Etcd的处理
	oldDeployment, err := c.clientSet.AppsV1().Deployments(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil && !apiErrors.IsNotFound(err) {
		utilRuntime.HandleError(err)
		return err
	}

	newDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: pointer.Int32Ptr(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"myapp": etcd.Spec.Name,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"myapp": etcd.Spec.Name,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  etcd.Spec.Name,
							Image: etcd.Spec.Image,
							Command: []string{
								"etcd",
								"--name=" + etcd.Spec.Name,
								"--data-dir=" + etcd.Spec.DataDir,
							},
						},
					},
				},
			},
		},
	}

	if apiErrors.IsNotFound(err) {
		_, err = c.clientSet.AppsV1().Deployments(namespace).Create(context.Background(), newDeployment, metav1.CreateOptions{})
		if err != nil {
			utilRuntime.HandleError(err)
			return err
		}
		c.recorder.Event(etcd, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	} else {
		newJson, err := json.Marshal(newDeployment)
		if err != nil {
			utilRuntime.HandleError(err)
			return err
		}

		oldJson, err := json.Marshal(oldDeployment)
		if err != nil {
			utilRuntime.HandleError(err)
			return err
		}

		patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldJson, newJson, appsv1.Deployment{})
		if err != nil {
			utilRuntime.HandleError(err)
			return err
		}

		deploy := &appsv1.Deployment{}
		err = json.Unmarshal(patchBytes, deploy)
		if err != nil {
			utilRuntime.HandleError(err)
			return err
		}

		_, err = c.clientSet.AppsV1().Deployments(namespace).Patch(context.Background(), name, types.MergePatchType, patchBytes, metav1.PatchOptions{})

		if err != nil {
			utilRuntime.HandleError(err)
			return err
		}
	}
	return nil
}

// enqueueEtcd 获取etcd resource，并将其转换为格式为: namespace/name string，然后将其放入workqueue。
// 除了etcd资源外之外，不应向该方法传递任何其他类型的资源。
func (c *Controller) enqueueEtcd(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		utilRuntime.HandleError(err)
		return
	}
	c.workerQueue.AddRateLimited(key)
}

// enqueueEtcdForDelete获取已删除的etcd resource，并将其转换为命名空间/名称字符串，然后将其放入workqueue。
// 除了etcd资源外之外，不应向该方法传递任何其他类型的资源。
func (c *Controller) enqueueEtcdForDelete(obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		utilRuntime.HandleError(err)
		return
	}
	c.workerQueue.AddRateLimited(key)
}
