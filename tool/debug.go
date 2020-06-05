package tool

import (
	"sync"
	"sync/atomic"
)

type Debug struct {
	DoDebug     bool          // 是否开启debug
	DeMu      sync.Mutex    // debug计数器锁
	Count     DebugCount    // debug计数结构体
}

// debug计数的内容
// Retry的数量与并发数量有关，一般小于等于并发数
// 如：
// N个并发获取许可，时间周期内许可被耗尽，N个并发等待，
// 下个周期开始时，清除上个周期channel，导致N个并发的等待失败，Retry
type Statistics struct {
	Wait  int32 // 等待获取许可数量
	Done  int32 // 许可派发数量
	Retry int32 // 重新获取许可数量
	Make  int32 // 生产的许可数量
}

// debug计数
type DebugCount map[int32]*Statistics

// debug 累加 当前时间周期中 获取许可等待数量
func (bc *Debug) AddWait(tag int32) {
	bc.DeMu.Lock()
	_, ok := bc.Count[tag]
	if !ok {
		bc.Count[tag] = &Statistics{}
	}
	atomic.AddInt32(&(bc.Count[tag].Wait), 1)
	bc.DeMu.Unlock()
}

// debug 累加 当前时间周期中 获取时刻失败重试数量
func (bc *Debug) AddRetry(tag int32) {
	bc.DeMu.Lock()
	atomic.AddInt32(&(bc.Count[tag].Retry), 1)
	bc.DeMu.Unlock()
}

// debug 累加 当前时间周期中 获取到许可的数量
func (bc *Debug) AddDone(tag int32) {
	bc.DeMu.Lock()
	atomic.AddInt32(&(bc.Count[tag].Done), 1)
	bc.DeMu.Unlock()
}

// debug 累加 当前时间周期中 生产的许可的数量
func (bc *Debug) SetMake(tag int32, num int32) {
	bc.DeMu.Lock()
	bc.Count[tag].Make = num
	bc.DeMu.Unlock()
}