package vGPUScheduler

import (
	"context"
	"encoding/json"
    "time"
	"errors"
	"fmt"

	v1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	"k8s.io/apimachinery/pkg/runtime"
	schedulerconfig "k8s.io/kube-scheduler/config/v1"
	"k8s.io/kubernetes/pkg/scheduler/framework"

	"sigs.k8s.io/scheduler-plugins/pkg/apis/config"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"vGPUScheduler/pkg/utils"
)

const (
	Name = "vGPUScheduler"
)

var (
	_ framework.FilterPlugin    = &vGPUScheduler{}
	_ framework.ScorePlugin     = &vGPUScheduler{}
    _ framework.ScoreExtensions = &vGPUScheduler{}
)

type vGPUScheduler struct {
    handle framework.Handle
    clientset *kubernetes.Clientset
}

func New(_ runtime.Object, handle framework.Handle) (framework.Plugin, error) {
    klog.V(5).InfoS("Alnair vGPU Scheduling plugin is enabled")

    cs, err := clientsetInit()
	if err != nil {
		return nil, fmt.Errorf("Alnair Cannot initialize in-cluster kubernetes config")
	}

	return &vGPUScheduler{
		handle: handle,
		clientset: cs,
	}, nil
}

func (g *vGPUScheduler) Name() string {
	return Name
}

func clientsetInit() (*kubernetes.Clientset, error) {
	// creates the in-cluster config
	var (
		config *rest.Config
		err    error
	)
	config, err = rest.InClusterConfig()
	if err != nil {
		config, err = clientcmd.BuildConfigFromFlags("", "/etc/kubernetes/scheduler.conf") //use the default path for now, pass through arg later
	}
	if err != nil {
		panic(err.Error())
	}
	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}
	return clientset, err

}

func UpdatePodAnnotations(clientset *kubernetes.Clientset, pod *v1.Pod) error {
	//dont use deep copy to newpod and update, will copy object version, cause the following error
	//err: Operation cannot be fulfilled on pods "XXX": the object has been modified; please apply your changes to the latest version and try again
	patchData := map[string]interface{}{"metadata": map[string]map[string]string{"annotations": {
		"scheduler-timestamp": fmt.Sprintf("%d", time.Now().UnixNano())}}}
	//patchData := {"metadata": {"annotations": {"Scheduler-TimeStamp": fmt.Sprintf("%d", time.Now().UnixNano())}}

	namespace := pod.Namespace
	podName := pod.Name

	playLoadBytes, _ := json.Marshal(patchData)
	_, err := clientset.CoreV1().Pods(namespace).Patch(context.TODO(), podName, types.StrategicMergePatchType, playLoadBytes, metav1.PatchOptions{})

	if err != nil {
		klog.V(5).ErrorS(err, "Alnair Pod Patch fail")
		return fmt.Errorf("Alnair %v pod Patch fail %v", podName, err)
	}

	return nil
}

func (g *vGPUScheduler) Filter(ctx context.Context, state *framework.CycleState, pod *v1.Pod, node *framework.NodeInfo) *framework.Status {
	klog.V(5).Infof("filter pod: %v, node: %v\n", pod.Name, node.Node().Name)
	// write timestamp to every pod which comes to here

	nodeinfos := utils.newNodeInfos(node.Node())
	if allocatable := nodeinfos.Assume(pod); allocatable {
	    return framework.NewStatus(framework.Success, "")
	}
	return framework.NewStatus(framework.Unschedulable, "Node:"+node.Node().Name)
}

// func PatchPodAnnotationSpec(oldPod *v1.Pod) (newPod *v1.Pod) {
// 	newPod = oldPod.DeepCopy()
// 	if len(newPod.ObjectMeta.Annotations) == 0 {
// 		newPod.ObjectMeta.Annotations = map[string]string{}
// 	}
// 	now := time.Now()
//     newPod.ObjectMeta.Annotations[EnvResourceAssumeTime] = fmt.Sprintf("%d", now.UnixNano())
// 	return newPod
// }

func (g *vGPUScheduler) Score(ctx context.Context, state *framework.CycleState, p *v1.Pod, nodeName string) (int64, *framework.Status) {
	// Get Node Info
	err := UpdatePodAnnotations(g.clientset, pod)
	if err != nil {
		return 0, framework.NewStatus(framework.Error, fmt.Sprintf("cannot patch timestamp to pod %s, err: %v", pod.Name, err))
	}

	klog.V(5).InfoS("Alnair add annotation to pod ", pod.Name)
	nodeInfo, err := g.handle.SnapshotSharedLister().NodeInfos().Get(nodeName)
	if err != nil {
		return 0, framework.NewStatus(framework.Error, fmt.Sprintf("getting node %q from Snapshot: %v", nodeName, err))
	}

    intScore, err := CalculateScore(nodeInfo)
	if err != nil {
		klog.V(5).Errorf("CalculateScore Error: %v", err)
		return 0, framework.NewStatus(framework.Error, fmt.Sprintf("Score Node: %v Error: %v", nodeInfo.Node().Name, err))
	}

    return intScore, framework.NewStatus(framework.Success)
}

func CalculateScore(info *framework.NodeInfo) uint64 {
    allocateMemorySum := uint64(0)
    for _, pod := range info.Pods {
		if mem, ok := pod.Pod.GetLabels()[ResourceName];ok {
			allocateMemorySum += StrToUint64(mem)
		}
	}
	return allocateMemorySum
}

func StrToUint64(str string) uint64 {
	if i, e := strconv.Atoi(str); e != nil {
		return 0
	} else {
		return uint64(i)
	}
}


func (g *vGPUScheduler) NormalizeScore(_ context.Context, _ *framework.CycleState, pod *v1.Pod, scores framework.NodeScoreList) *framework.Status {
	var (
		highest int64 = 0
		lowest        = scores[0].Score
	)

	for _, nodeScore := range scores {
		if nodeScore.Score < lowest {
			lowest = nodeScore.Score
		}
		if nodeScore.Score > highest {
			highest = nodeScore.Score
		}
	}

	if highest == lowest {
		lowest--
	}

	// Set Range to [0-100]
	for i, nodeScore := range scores {
		scores[i].Score = (nodeScore.Score - lowest) * framework.MaxNodeScore / (highest - lowest)
		klog.V(5).Infof("Node: %v, Score: %v in Plugin: when scheduling Pod: %v/%v", scores[i].Name, scores[i].Score, pod.GetNamespace(), pod.GetName())
	}
	return framework.NewStatus(framework.Success)
}

func (g *vGPUScheduler) ScoreExtensions() framework.ScoreExtensions {
	return g
}
