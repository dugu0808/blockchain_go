package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"math/rand"
	"time"
)

//区块结构
type Block struct {
	Index     int
	Timestamp string
	BPM       int
	Hash      string
	PrevHash  string
	Delegate  string
}

//生成区块
func generateBlock(oldBlock Block, BPM int, address string) (Block, error) {
	var newBlock Block

	t := time.Now()

	newBlock.Index = oldBlock.Index + 1
	newBlock.Timestamp = t.String()
	newBlock.BPM = BPM
	newBlock.PrevHash = oldBlock.Hash
	newBlock.Hash = caculateBlockHash(newBlock)
	newBlock.Delegate = address

	return newBlock, nil
}

//区块链
var Blockchain []Block

//委托人
var delegates = []string{"001", "002", "003", "004", "005"}

//当前的delegates的索引
var indexDelegate int

//生成Hash字符串
func generateHash(s string) string {
	h := sha256.New()
	h.Write([]byte(s))
	hashed := h.Sum(nil)
	return hex.EncodeToString(hashed)
}

//生成区块的Hash
func caculateBlockHash(block Block) string {
	record := string(block.Index) + block.Timestamp + string(block.BPM) + block.PrevHash
	return generateHash(record)
}

func isBlockValid(newBlock, oldBlock Block) bool {
	if oldBlock.Index+1 != newBlock.Index {
		return false
	}

	if oldBlock.Hash != newBlock.PrevHash {
		return false
	}

	if caculateBlockHash(newBlock) != newBlock.Hash {
		return false
	}

	return true
}

//更换委托人排列顺序
func randDelegate(delegates []string) []string {
	var randList []string
	randList = delegates[1:]
	randList = append(randList, delegates[0])

	fmt.Printf("%v\n", randList)
	return randList
}

func main() {
	indexDelegate = 0

	//创世区块
	t := time.Now()
	genesisBlock := Block{}
	genesisBlock = Block{0, t.String(), 0, caculateBlockHash(genesisBlock), "", ""}
	Blockchain = append(Blockchain, genesisBlock)

	indexDelegate++

	countDelegate := len(delegates)

	for indexDelegate < countDelegate {
		//3秒出一个块
		time.Sleep(time.Second * 3)

		fmt.Println(indexDelegate)

		//出块
		rand.Seed(int64(time.Now().Unix()))
		bpm := rand.Intn(100)
		oldLastIndex := Blockchain[len(Blockchain)-1]
		newBlock, err := generateBlock(oldLastIndex, bpm, delegates[indexDelegate])
		if err != nil {
			log.Println(err)
			continue
		}

		fmt.Printf("Blockchain....%v\n", newBlock)

		if isBlockValid(newBlock, oldLastIndex) {
			Blockchain = append(Blockchain, newBlock)
		}

		indexDelegate = (indexDelegate + 1) % countDelegate

		if indexDelegate == 0 {
			//更换委托人的排列顺序
			delegates = randDelegate(delegates)
		}
	}
}
