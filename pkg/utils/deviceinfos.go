package utils

import (
	"log"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"strconv"
	"time"
)

// DeviceInfos is pod level aggregated information

type DeviceInfos struct {
	idx    int
	podMap map[types.UID]*v1.Pod
// 	usedGPUMem  uint
	totalGPUMem uint
}

func newDeviceInfos(index int, totalGPUMem uint) *DeviceInfos {
	return &DeviceInfos{
		idx:         index,
		totalGPUMem: totalGPUMem,
		podMap:      map[types.UID]*v1.Pod{},
	}
}



func (d *DeviceInfos) GetUsedGPUMemory() (gpuMem uint) {
	log.Printf("debug: GetUsedGPUMemory() podMap %v, and its address is %p", d.podMap, d)
// 	d.rwmu.RLock()
// 	defer d.rwmu.RUnlock()
	for _, pod := range d.podMap {
	    if pod.Status.Phase == v1.PodRunning || pod.Status.Phase == v1.PodPending{
	        log.Printf("debug: the pod %s in ns %s is counted as used due to its status in %s", pod.Name, pod.Namespace, pod.Status.Phase)
	        gpuMem += GetGPUMemoryFromPodAnnotation(pod)
	}
	return gpuMem
}

// GetGPUMemoryFromPodAnnotation gets the GPU Memory of the pod
func GetGPUMemoryFromPodAnnotation(pod *v1.Pod) (gpuMemory uint) {
	if len(pod.ObjectMeta.Annotations) > 0 {
		vGPU_ID, found := pod.ObjectMeta.Annotations["alnair-gpu-id"]
		idx := GetvGPUIDX(vGPU_ID)
		if found {
			s, _ := len(idx)
			if s < 0 {
				s = 0
			}
			gpuMemory += uint(s)
		}
	}
	log.Printf("debug: pod %s in ns %s with status %v has vGPU Mem %d",
		pod.Name,
		pod.Namespace,
		pod.Status.Phase,
		gpuMemory)
	return gpuMemory
}

// get vGPU index
func GetvGPUIDX(vGPU_ID []string) []string {
    var ret []string
    for _, sid := range vGPU_ID {
        id := string.SplitN(sid, "_", 2)[1]
        if len(ret) == 0 || id != ret[len(ret)-1]{
            ret = append(ret, id)
        }
    }
    return ret
}