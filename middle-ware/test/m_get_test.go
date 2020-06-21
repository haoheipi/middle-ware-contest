package test

import (
	"bufio"
	"fmt"
	"io"
	"middle-ware/util"
	"os"
	"sync"
	"testing"
	"time"
)

func TestRead(T *testing.T) {
	blockPtrChannel := make(chan *[]byte, 64)
	//runtime.GOMAXPROCS(2)
	BatchCount := 1024
	var BatchTraceList []map[string]*[]string
	for i := 0; i < BatchCount; i++ {
		BatchTraceList = append(BatchTraceList, make(map[string]*[]string, 8192))
	}
	var wg2 sync.WaitGroup
	wg2.Add(1)

	//开始处理数据
	start := time.Now()

	//dataPath := "http://localhost:8080/trace1.data"
	//resp, err := http.Get(dataPath)
	//if err != nil {
	//	panic(err)
	//}

	// 监听channel数据
	go func() {
		//count := 0
		//pos := 0
		//set<string>
		//BadTraceIdList := make([]byte, 0, 512)
		//traceMap := BatchTraceList[pos]
		//badcount := 0
		remainByte := make([]byte, 0, 512)
		for {
			blockPtr, err := <-blockPtrChannel
			time.Sleep(time.Second * 1)
			if !err {
				fmt.Println("结束")
				break
			}
			block := *blockPtr
			index := 0
			i := 0
			for i = 0; i < len(block); i++ {
				if block[i] == '\n' {
					line := append(remainByte, block[index:i]...)
					i++
					index = i
					fmt.Println("first line:" + util.Bytes2str(line))
					break
				}
			}
			for ; i < len(block); i++ {
				if block[i] == '\n' {
					//line := block[index:i]
					index = i + 1
					//fmt.Printf(util.Bytes2str(line))
				}
			}
			remainByte = block[index:]
			//fmt.Println(util.Bytes2str(remainByte))
		}
	}()

	f, _ := os.Open("/Users/hhp/Documents/contest/middle-ware/error.log")

	fmt.Println("start handle")
	r := bufio.NewReaderSize(f, 1024*1024*256) //todo 调整数值可能有优化
	for {
		block := make([]byte, 3)
		_, err := io.ReadFull(r, block)
		fmt.Println(block)
		blockPtrChannel <- &block
		if err != nil {
			fmt.Println("Error reading file:", err)
			close(blockPtrChannel)
			break
		}
	}
	end := time.Now()
	fmt.Println(end.Sub(start))
	wg2.Wait()
}
