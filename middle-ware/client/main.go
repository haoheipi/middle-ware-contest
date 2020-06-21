package main

import (
	"bufio"
	"errors"
	"io"
	"log"
	"middle-ware/util"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"
)

var BatchTraceList []map[string]*[]string
var BatchCount int
var ErrBufferFull = errors.New("bufio: buffer full")
var logger *log.Logger

var count = 0
var pos = 0
var BadTraceIdList = make([]byte, 0, 512)
var badCount = 0

func main() {
	log.Println("Starting the client ...")
	//runtime.GOMAXPROCS(2)
	BatchCount = 1024
	for i := 0; i < BatchCount; i++ {
		BatchTraceList = append(BatchTraceList, make(map[string]*[]string, 8192))
	}
	blockPtrChannel := make(chan *[]byte, 64)

	var wg sync.WaitGroup

	args := os.Args
	var path string
	if args[1] == util.ClientProcessPort1 {
		path = "/trace1.data"
	} else if args[1] == util.ClientProcessPort2 {
		path = "/trace2.data"
	}

	//启动ready和setParameter服务
	launchClient := NewLaunchClient()
	wg.Add(1)
	go func() {
		defer wg.Done()
		launchClient.LaunchClient(args[1])
	}()
	wg.Wait()

	// 监听channel数据
	go func() {
		remainByte := make([]byte, 0, 512)
		for {
			blockPtr, exist := <-blockPtrChannel
			tmpStart := time.Now()
			if !exist {
				log.Printf("count:%d\n", count)
				log.Printf("badcount:%d\n", badCount)
				launchClient.UpdateWrongTraceIdThread(BadTraceIdList, count/util.BatchSize)
				launchClient.SetWTIConn.Write([]byte("!" + strconv.Itoa(count/util.BatchSize)))
				break
			}
			block := *blockPtr
			index := 0
			i := 0
			for i = 0; i < len(block); i++ {
				if block[i] == '\n' {
					line := append(remainByte, block[index:i+1]...)
					launchClient.handlerLine(line)
					i++
					index = i
					break
				}
			}
			for ; i < len(block); i++ {
				if block[i] == '\n' {
					launchClient.handlerLine(block[index:i+1])
					index = i + 1
				}
			}
			remainByte = block[index:]
			tmpEnd := time.Now()
			log.Println("hand one block time:", tmpEnd.Sub(tmpStart))
		}
	}()

	var wg2 sync.WaitGroup
	wg2.Add(1)
	//开始处理数据
	start := time.Now()
	dataPath := "http://localhost:" + launchClient.TraceDataPort + path
	//dataPath := "http://localhost:8080" + path
	resp, err := http.Get(dataPath)
	if err != nil {
		panic(err)
	}

	log.Println("start handle")
	r := bufio.NewReaderSize(resp.Body, 1024*1024*256) //todo 调整数值可能有优化
	for {
		block := make([]byte, 1024*1024*256)
		tmpStart := time.Now()
		_, err := io.ReadFull(r, block)
		tmpEnd := time.Now()
		log.Println("download one block time:", tmpEnd.Sub(tmpStart))
		blockPtrChannel <- &block
		if err != nil {
			log.Println("Error reading file:", err)
			close(blockPtrChannel)
			break
		}
	}

	end := time.Now()
	log.Println("download sum time:", end.Sub(start))
	wg2.Wait()
}
