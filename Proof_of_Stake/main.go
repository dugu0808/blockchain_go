package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/joho/godotenv"
)

//每个区块的内容
type Block struct {
	Index     int
	Timestamp string
	BPM       int
	Hash      string
	PreHash   string
	Validator string
}

//区块链，经过验证的区块的集合
var Blockchain []Block

//临时存储区块，区块被选出来添加到Blockchain之前临时存储
var tempBlocks []Block

//Block通道，任意节点提出新块时发送到该通道
var candidateBlocks = make(chan Block)

//向所有节点广播最新的区块链
var announcements = make(chan string)

//互斥锁
var mutex = &sync.Mutex{}

//节点存储和token余额的记录
var validators = make(map[string]int)

//生成区块
func generateBlock(oldBlock Block, BPM int, address string) (Block, error) {
	var newBlock Block
	t := time.Now()

	newBlock.Index = oldBlock.Index + 1
	newBlock.Timestamp = t.String()
	newBlock.BPM = BPM
	newBlock.PreHash = oldBlock.Hash
	newBlock.Hash = caculateBlockHash(newBlock)
	newBlock.Validator = address //记录节点地址

	return newBlock, nil
}

//SHA256计算哈希
func caculateHash(s string) string {
	h := sha256.New()
	h.Write([]byte(s))
	hashed := h.Sum(nil)
	return hex.EncodeToString(hashed)
}

//计算区块哈希
func caculateBlockHash(block Block) string {
	record := string(block.Index) + block.Timestamp + string(block.BPM) + block.PreHash
	return caculateHash(record)
}

//验证区块。主要验证递增的区块索引Index，PreHash以及Hash
func isBlockValid(newBlock, oldBlock Block) bool {
	if oldBlock.Index+1 != newBlock.Index {
		return false
	}
	if oldBlock.Hash != newBlock.PreHash {
		return false
	}
	if caculateBlockHash(newBlock) != newBlock.Hash {
		return false
	}

	return true
}

//验证者
func handleConn(conn net.Conn) {
	defer conn.Close()

	go func() {
		for {
			msg := <-announcements
			io.WriteString(conn, msg)
		}
	}()

	//验证者地址
	var address string

	//验证者输入所拥有的token数，影响获得新区块记账权的概率
	io.WriteString(conn, "Enter token balance:")
	scanBalance := bufio.NewScanner(conn)
	for scanBalance.Scan() {
		//获取输入的数据，并转为int型
		balance, err := strconv.Atoi(scanBalance.Text())
		if err != nil {
			log.Printf("%v not a number: %v", scanBalance.Text(), err)
			return
		}
		t := time.Now()
		//生成验证者地址
		address = caculateHash(t.String())
		//存储验证者的地址和token
		validators[address] = balance
		fmt.Println(validators)
		break
	}

	io.WriteString(conn, "\nEnter a new BPM")
	scanBPM := bufio.NewScanner(conn)

	go func() {
		for {
			for scanBPM.Scan() {
				bpm, err := strconv.Atoi(scanBPM.Text())
				//如果验证者试图提议一个被污染的block(例BPM非整数)，从validators中删除该验证者，同时失去抵押的token
				if err != nil {
					log.Printf("%v not a number: %v", scanBPM.Text(), err)
					delete(validators, address)
					conn.Close()
				}

				mutex.Lock()
				oldLastIndex := Blockchain[len(Blockchain)-1]
				mutex.Unlock()

				//创建新的区块，然后发送到candidateBlocks通道
				newBlock, err := generateBlock(oldLastIndex, bpm, address)
				if err != nil {
					log.Println(err)
					continue
				}
				if isBlockValid(newBlock, oldLastIndex) {
					candidateBlocks <- newBlock
				}
				io.WriteString(conn, "\nEnter a new BPM:")
			}
		}
	}()

	//周期性打印最新区块链信息
	for {
		time.Sleep(time.Minute)
		mutex.Lock()
		output, err := json.Marshal(Blockchain)
		mutex.Unlock()
		if err != nil {
			log.Fatal(err)
		}
		io.WriteString(conn, string(output) + "\n")
	}
}

//选出获得记账权的节点，被选中的概率与抵押的tokens数量有关
func pickWinner() {
	//每隔30秒进行一次选取
	time.Sleep(30 * time.Second)
	mutex.Lock()
	temp := tempBlocks
	mutex.Unlock()
	
	lotteryPool := []string{}
	if len(temp) > 0 {
	
	OUTER:
		for _, block := range temp {
			//如果在lottery pool中存在和temp暂存区域里有相同的验证者，则跳过
			for _, node := range lotteryPool {
				if block.Validator == node {
					continue OUTER
				}
			}

			mutex.Lock()
			setValidators := validators
			mutex.Unlock()

			//获取验证者的tokens
			k, ok := setValidators[block.Validator]
			if ok {
				//向lotteryPool中写入k条数据，k为验证者的tokens，进而影响了被选中的概率
				for i := 0; i < k; i++ {
					lotteryPool = append(lotteryPool, block.Validator)
				}
			}
			
		}

		//随机选出获胜节点的地址
		s := rand.NewSource(time.Now().Unix())
		r := rand.New(s)
		lotteryWinner := lotteryPool[r.Intn(len(lotteryPool))]

		//把选出的节点的区块添加到整条区块链上，然后通知所有节点胜利者的信息
		for _, block := range temp {
			if block.Validator == lotteryWinner {
				mutex.Unlock()
				for _ = range validators {
					announcements <- "\nwinning validators: " + lotteryWinner + "\n"
				}
				break
			}
		}
	}

	mutex.Lock()
	tempBlocks = []Block{}
	mutex.Unlock()
	
}


func main() {
	//godotenv.Load() 会解析 .env 文件并将相应的Key/Value对都放到环境变量中，通过 os.Getenv 获取
	err := godotenv.Load()
	if err != nil {
		log.Fatal(err)
	}

	//生成创世区块
	t := time.Now()
	genesisBlock := Block{}
	genesisBlock = Block{0, t.String(), 0, caculateBlockHash(genesisBlock), "", ""}
	spew.Dump(genesisBlock)
	Blockchain = append(Blockchain, genesisBlock)
	
	httpPort := os.Getenv("PORT")

	//启动TCP服务
	server, err := net.Listen("tcp", ":" + httpPort)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("HTTP Server Listening on port :", httpPort)
	defer server.Close()

	//启动一个Go routine从candidateBlocks通道中获取提议的区块，然后填充到临时缓冲区tempBlocks中
	go func() {
		for candidate := range candidateBlocks {
			mutex.Lock()
			tempBlocks = append(tempBlocks, candidate)
			mutex.Unlock()
		}
	}()

	//启动了一个Go routine完成pickWinner函数
	go func() {
		for {
			pickWinner()
		}
	}()

	//接收验证者节点的连接
	for {
		conn, err := server.Accept()
		if err != nil {
			log.Fatal(err)
		}
		go handleConn(conn)
	}
}