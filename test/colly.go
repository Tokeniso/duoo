package main

import (
	"fmt"
	"rate/rate"
	"time"

	"github.com/gocolly/colly/v2"
	"github.com/gocolly/colly/v2/queue"
)

// 基于colly的测试
func main() {
	c := colly.NewCollector(
		colly.AllowURLRevisit(),
		colly.Async(false), // request过多时，异步模式会无限制开启goroutine，导致崩溃，用同步+队列控制请求速率，规避cc defense
	)
	c.SetRequestTimeout(5 * time.Second)
	threads := 3 // 线程数

	// ******  colly旧版队列 Run 存在无限循环bug， 官方已修复 github issue #450
	store := queue.InMemoryQueueStorage{
		MaxSize: 100000000,
	}
	qu, _ := queue.New(threads, &store)

	var err error

	for i := 0; i < 2000; i++ {
		err = qu.AddURL("http://www.github.com/")
	}

	/** 初始化 **/
	var number int32 = 500 // 每second许可数量
	var second int32 = 3   // 秒

	boolRate := rate.NewRate(number, second)
	boolRate.SetDebug()

	c.OnRequest(func(request *colly.Request) {
		// 同步+队列模式下，控制请求速率，拿到许可才能请求
		_, err := boolRate.GetPermission()
		if err != nil {
			// 异常处理
		}
		request.Abort() // 不实际请求
	})

	// 请求使用goroutine 时 队列只是加快把请求全部投递至新建的goroutine
	// 请求使用同步时才显示threads的作用，开启 threads 个goroutine 执行request
	err = qu.Run(c)
	// 等待所有的request goroutine执行完毕
	c.Wait()

	for key, val := range boolRate.Count {
		fmt.Printf("循环批次：%d -- wait：%d -- retry：%d -- done：%d\n", key, val.Wait, val.Retry, val.Done)
	}
	fmt.Println(err)
}
