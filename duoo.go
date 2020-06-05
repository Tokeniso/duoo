package duoo

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Tokeniso/duoo/emitter"
	"github.com/Tokeniso/duoo/tool"
)

const (
	runningStatus = iota // 运行
	stopStatus    = iota // 停止
)

// 存储时间周期对应的channel    map[时间周期Tag]当前时间周期channel
type rateChan map[int32]chan interface{}

// 终止速率分配
type stopAction struct {
	status int32     // 服务状态 runningStatus 运行 stopStatus 被停止
	ch     chan bool // 停止定时器
	cancel func()    // 停止许可分配ctx
}

// ·布尔型速率控制·结构体
// 可修改ch的类型来传递更多实际的```许可令牌```
// 每 bc.second 秒最多派发 bc.rate 个许可，超过时间周期会被清空
type RateEmitter struct {
	rate          int32         // 时间周期内许可的数量
	second        int32         // 许可的时间周期
	ch            rateChan      // 许可channel(布尔类型)
	chanMax       int32         // ch 最大长度
	isNew         bool          // 新建channel的派发状态， true未派发，false已派发
	start         time.Time     // 速率控制启动时间
	fail          int           // 失败重试次数
	mu            sync.Mutex    // channel读写锁
	timeCycle     time.Duration // 时间周期对应的 time.Duration
	down          stopAction    // 停止通知

	emitter       emitter.Emitter
	tool.Debug
}

// 初始化一个·自定义速率控制·
func newRate(rate int32, second int32, ei emitter.Emitter) *RateEmitter {
	bc := &RateEmitter{}
	bc.rate = rate
	bc.second = second
	bc.ch = make(rateChan)
	bc.chanMax = 100000
	bc.isNew = false
	bc.start = time.Now()
	bc.fail = 5
	bc.mu = sync.Mutex{}
	// debug
	bc.DeMu = sync.Mutex{}
	bc.Count = tool.DebugCount{}
	bc.DoDebug = false

	bc.emitter = ei

	bc.timeCycle = time.Duration(bc.second) * time.Second
	bc.down = stopAction{runningStatus, make(chan bool), func() {}}
	return bc
}

// 启动一个·布尔型速率控制·
func RateStart(rate int32, second int32) *RateEmitter {
	ei := emitter.BoolEmitter{}
	bc := newRate(rate, second, &ei)
	bc.beginTicker()

	return bc
}

// 启动一个·自定义速率控制·
func StartEmitter(rate int32, second int32, ei emitter.Emitter) *RateEmitter {
	bc := newRate(rate, second, ei)
	bc.beginTicker()

	return bc
}

// 启动一个带debug的·布尔型速率控制·
func RateStartWithDebug(rate int32, second int32) *RateEmitter {
	fmt.Println("启动中....")
	ei := emitter.BoolEmitter{}
	bc := newRate(rate, second, &ei)
	bc.SetDebug()
	bc.beginTicker()
	fmt.Println("启动完成.")
	return bc
}

// 启动一个带debug的·自定义速率控制·
func StartEmitterWithDebug(rate int32, second int32, ei emitter.Emitter) *RateEmitter {
	fmt.Println("启动中....")
	bc := newRate(rate, second, ei)
	bc.SetDebug()
	bc.beginTicker()
	fmt.Println("启动完成.")
	return bc
}

// 设置debug模式
func (bc *RateEmitter) SetDebug() {
	bc.DoDebug = true
}

// 设置chan最大长度
func (bc *RateEmitter) SetChanMax(i int32) {
	bc.chanMax = i
}

// 设置最大失败次数
func (bc *RateEmitter) SetFail(number int) {
	bc.fail = number
}

// 新建一个channel
func (bc *RateEmitter) recoverChan() chan interface{} {
	max := bc.chanMax
	if bc.rate < bc.chanMax {
		max = bc.rate
	}
	return make(chan interface{}, max)
}

// 从0获取一个许可
func (bc *RateEmitter) GetPermission() (interface{}, error) {
	return bc.Permission(0)
}

// 获取一个许可
func (bc *RateEmitter) Permission(fail int) (interface{}, error) {
	if bc.down.status == stopStatus {
		return false, errors.New("service stopped")
	}
	// 竞争失败下不做处理
	if bc.fail > 0 && bc.fail < fail {
		return false, errors.New("over the maximum of failed")
	}
	ch, tag, _ := bc.getChanByStore()
	if bc.DoDebug {
		bc.AddWait(tag)
	}
	data, col := bc.queuePermission(ch)
	if !col { // 许可channel已关闭，重试
		if bc.DoDebug {
			bc.AddRetry(tag)
		}
		fail++
		return bc.Permission(fail)
	}
	if bc.DoDebug {
		bc.AddDone(tag)
	}
	return data, nil
}

// 准备创建channel
func (bc *RateEmitter) prepareMakeChan() {
	ctx, call := context.WithTimeout(context.Background(), bc.timeCycle)
	bc.down.cancel = call
	bc.makeChan(ctx)
}

// 给未派发的channel派发许可
func (bc *RateEmitter) makeChan(ctx context.Context) {
	ch, tag, ok := bc.getChanByCreate()
	if !ok {
		return
	}
	if bc.down.status == stopStatus {
		return
	}
	if bc.DoDebug {
		fmt.Println("refresh channel, tag:", tag)
	}

	bc.makePellets(ch, tag, ctx)
}

// 执行循环生产
func (bc *RateEmitter) makePellets(ch chan interface{}, tag int32, ctx context.Context) {
	var i int32
	rate := bc.rate
	for i = 0; i < rate; i ++ {
		select {
		case <-ctx.Done(): // 超过channel的时间周期关闭派发许可
			rate = i
			break
		default:
			bc.emitter.MakePellets(ch, tag)
		}
	}
	if bc.DoDebug {
		bc.Debug.SetMake(tag, i)
	}
}

// 获取当前时间周期的channel，不检测派发状态
func (bc *RateEmitter) getChanByStore() (chan interface{}, int32, bool) {
	return bc.chanFactory(false)
}

// 获取当前时间周期的channel，检测派发状态
func (bc *RateEmitter) getChanByCreate() (chan interface{}, int32, bool) {
	return bc.chanFactory(true)
}

// channel生产者    返回当前时间周期中的channel(无则创建，创建时标记isNew)并关闭以往的channel
// bc.Permission() 调用时 bool 返回参数可忽略
// bc.makeChan()   调用时 bool 返回参数 true => "需要对channel进行许可派发"  false => "已被执行许可派发"
func (bc *RateEmitter) chanFactory(create bool) (chan interface{}, int32, bool) {
	bc.mu.Lock()
	defer func() {
		bc.mu.Unlock()
	}()
	tag := bc.GetTag()
	ch, ok := bc.ch[tag]
	if !ok { // channel不存在，清理旧channel
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
func (bc *RateEmitter) queuePermission(ch chan interface{}) (interface{}, bool) {
	if bc.down.status == stopStatus {
		return false, false
	}
	data, col := <-ch
	return data, col
}

// 获取当前时间周期内的Tag
func (bc *RateEmitter) GetTag() int32 {
	ti := time.Now().Sub(bc.start).Seconds()
	tag := int32(ti) / bc.second
	return tag
}

// 关闭所有存在的channel
func (bc *RateEmitter) clearChan() {
	defer func() {
		if recover() != nil {
			// TODO error log 忽略往关闭的chan错误
		}
	}()

	// 时间周期结束强制关闭通道，防止数据溢出
	for _, ch := range bc.ch {
		close(ch)
	}
	bc.ch = make(rateChan)
}

// 启动派发channel许可的计时器
func (bc *RateEmitter) beginTicker() {
	go func() {
		// 启动就开始分配
		bc.prepareMakeChan()
		ticker := time.NewTicker(bc.timeCycle)
		for {
			select {
			case <-ticker.C:
				bc.prepareMakeChan()
			case <-bc.down.ch:
				ticker.Stop()
				time.AfterFunc(bc.timeCycle, func() { // 为了chanFactory 中的清理工作
					bc.prepareMakeChan()
				})
				return
			}
		}
	}()
}

// 停止许可派发，并从下一个时间周期开始终止速率分配
// 注：没有在一个时间周期中就终止速率分配是因为并发下会出现不可预期的死锁问题
func (bc *RateEmitter) Stop() {
	if atomic.AddInt32(&bc.down.status, 1) == stopStatus {
		bc.down.ch <- true // 通知定时器停止
		bc.down.cancel()   // 关闭许可分配ctx
	} else {
		atomic.AddInt32(&bc.down.status, -1)
	}
}
