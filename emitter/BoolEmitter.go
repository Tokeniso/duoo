package emitter

type BoolEmitter struct {

}

func (be *BoolEmitter) MakePellets(ch chan interface{}, data interface{}) interface{} {
	defer func() {
		if recover() != nil {
			// TODO error log 忽略往关闭的chan中投放数据
		}
	}()
	ch <- true
	return true
}