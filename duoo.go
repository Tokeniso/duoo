package duoo

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// 存储时间周期对应的channel    map[时间周期Tag]当前时间周期channel
type rateChan map[int32]chan bool

// debug计数
type debugCount map[int32]*statistics

// debug计数的内容
// Retry的数量与并发数量有关，一般小于等于并发数
// 如：
// N个并发获取许可，时间周期内许可被耗尽，N个并发等待，
// 下个周期开始时，清除上个周期channel，导致N个并发的等待失败，Retry
type statistics struct {
	Wait  int32    // 等待获取许可数量
	Done  int32    // 许可派发数量
	Retry int32    // 重新获取许可数量
}

// ·布尔型速率控制·结构体
// 可修改ch的类型来传递更多实际的```许可令牌```
// 每 bc.second 秒最多派发 bc.rate 个许可，超过时间周期会被清空
type BoolControl struct {
	rate 		int32 			// 时间周期内许可的数量
	second  	int32 			// 许可的时间周期
	ch 	    	rateChan 		// 许可channel(布尔类型)
	isNew  		bool 			// 新建channel的派发状态， true未派发，false已派发
	start 		time.Time 		// 速率控制启动时间
	fail 		int      		// 失败重试次数
	mu 			sync.Mutex 		// channel读写锁
	chanMu 		sync.Mutex		// 许可派发同步锁
	deMu 		sync.Mutex 		// debug计数器锁
	Count   	debugCount 		// debug计数结构体
	debug   	bool 			// 是否开启debug
	timeCycle 	time.Duration   // 时间周期对应的 time.Duration
}

// 初始化一个·布尔型速率控制·
func NewRate(rate int32, second int32) *BoolControl {
	bc := &BoolControl{}
	bc.rate = rate
	bc.second = second
	bc.ch = make(rateChan)
	bc.isNew = false
	bc.start = time.Now()
	bc.fail = 5
	bc.mu = sync.Mutex{}
	bc.chanMu = sync.Mutex{}
	bc.deMu = sync.Mutex{}
	bc.Count = debugCount{}
	bc.debug = false
	bc.timeCycle = time.Duration(bc.second) * time.Second
	atomic.AddInt32(&bc.rate, 1)
	fmt.Println("启动中....")
	bc.Start()
	_, _ = bc.GetPermission()
	fmt.Println("启动完成.")
	atomic.AddInt32(&bc.rate, -1)
	return bc
}

// 设置debug模式
func (bc *BoolControl) SetDebug() {
	bc.debug = true
}

// 新建一个channel
func (bc *BoolControl) recoverChan() chan bool {
	return make(chan bool, bc.rate)
}

// 从0获取一个许可
func (bc *BoolControl) GetPermission() (bool, error) {
	return bc.Permission(0)
}

// 获取一个许可
func (bc *BoolControl) Permission(fail int) (bool, error) {
	if bc.fail < fail {
		return false, errors.New("over the maximum of failed")
	}
	ch, tag, _ := bc.getChanByStore()
	if bc.debug {
		bc.addWait(tag)
	}
	data, col := bc.queuePermission(ch)
	if !col { // 许可channel已关闭，重试
		if bc.debug {
			bc.addRetry(tag)
		}
		fail++
		return bc.Permission(fail)
	}
	if bc.debug {
		bc.addDone(tag)
	}
	return data, nil
}

// 准备创建channel
func (bc *BoolControl) prepareMakeChan() {
	ctx, _ := context.WithTimeout(context.Background(), bc.timeCycle)
	bc.makeChan(ctx)
}

// 给未派发的channel派发许可
func (bc *BoolControl) makeChan(ctx context.Context) {
	ch, _, ok := bc.getChanByCreate()
	if !ok {
		return
	}
	var i int32
	for i = 0; i < bc.rate; i ++ {
		select {
		case <-ctx.Done(): // 超过channel的时间周期关闭派发许可
			return
		default:
			ch <- true
		}
	}
}

// 获取当前时间周期的channel，不检测派发状态
func (bc *BoolControl) getChanByStore() (chan bool, int32, bool) {
	return bc.chanFactory(false)
}

// 获取当前时间周期的channel，检测派发状态
func (bc *BoolControl) getChanByCreate() (chan bool, int32, bool) {
	return bc.chanFactory(true)
}

// channel生产者    返回当前时间周期中的channel(无则创建，创建时标记isNew)并关闭以往的channel
// bc.Permission() 调用时 bool 返回参数可忽略
// bc.makeChan()   调用时 bool 返回参数 true => "需要对channel进行许可派发"  false => "已被执行许可派发"
func (bc *BoolControl) chanFactory(create bool) (chan bool, int32, bool) {
	bc.mu.Lock()
	defer func() {
		bc.mu.Unlock()
	}()
	tag := bc.getTag()
	ch, ok := bc.ch[tag]
	if !ok { // channel不存在
		bc.clearChan()
		ch = bc.recoverChan()
		bc.ch[tag] = ch
		bc.isNew = true
	}
	if create && bc.isNew { // 需要对channel发送许可
		bc.isNew = false
		return ch, tag, true
	}
	return ch, tag, false
}

// 许可派发
func (bc *BoolControl) queuePermission(ch chan bool) (bool, bool) {
	bc.chanMu.Lock()
	defer func() {
		bc.chanMu.Unlock()
	}()
	data, col := <-ch
	return data, col
}

// 获取当前时间周期内的Tag
func (bc *BoolControl) getTag() int32 {
	ti := time.Now().Sub(bc.start).Seconds()
	tag := int32(int32(ti) / bc.second)
	return tag
}

// 关闭所有存在的channel
func (bc *BoolControl) clearChan() {
	defer func() {
		if recover() != nil {
			// ignore the panic of close channel
		}
	}()

	for _, ch := range bc.ch {
		close(ch)
	}
	bc.ch = make(rateChan)
}

// 启动派发channel许可的计时器
func (bc *BoolControl) Start() {
	go func() {
		ticker := time.NewTicker(bc.timeCycle)
		for {
			<-ticker.C
			fmt.Println("refresh channel")
			bc.prepareMakeChan()
		}
	}()
}

// debug 累加 当前时间周期中 获取许可等待数量
func (bc *BoolControl) addWait(tag int32) {
	bc.deMu.Lock()
	_, ok := bc.Count[tag]
	if !ok {
		bc.Count[tag] = &statistics{}
	}
	atomic.AddInt32(&(bc.Count[tag].Wait), 1)
	bc.deMu.Unlock()
}

// debug 累加 当前时间周期中 获取时刻失败重试数量
func (bc *BoolControl) addRetry(tag int32) {
	bc.deMu.Lock()
	atomic.AddInt32(&(bc.Count[tag].Retry), 1)
	bc.deMu.Unlock()
}

// debug 累加 当前时间周期中 获取到许可的数量
func (bc *BoolControl) addDone(tag int32) {
	bc.deMu.Lock()
	atomic.AddInt32(&(bc.Count[tag].Done), 1)
	bc.deMu.Unlock()
}
