package emitter

// 速率生产接口
// MakePellets必须使用recover屏蔽错误，不然高并发会panic: send on closed channel
type Emitter interface {
	MakePellets(ch chan interface{}, data interface{}) interface{}
}
