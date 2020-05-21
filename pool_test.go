package workpool

import (
	"log"
	"testing"
)

func TestPool(t *testing.T) {
	task := NewTask(func() error {
		log.Println("hello")
		return nil
	})

	//p := NewPool(3, 100)
	p := NewPool(3, 0, 100)
	//p := NewPool(3, 0)
	go func() {
		i := 0
		for {
			//if i > 100 {
			//	break
			//}

			p.AddTask(task)
			i++
		}
	}()

	p.Run()
}

/**
go test -v
2020/05/21 23:47:52 current worker id:  1
2020/05/21 23:47:52 received signal:  interrupt
2020/05/21 23:47:52 hello
2020/05/21 23:47:52 task will exit...
2020/05/21 23:47:52 current worker id:  0
2020/05/21 23:47:52 hello
2020/05/21 23:47:52 current worker id:  2
2020/05/21 23:47:52 hello
2020/05/21 23:47:52 current worker id:  1
--- PASS: TestPool (1.86s)
PASS
ok  	github.com/daheige/workpool	1.865s
*/
