package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcutil"
)

var (
	dbDir           = ""
	gcPercent       = uint(10)
	cpu             = 0
	addrMap         = map[string]bool{"p2pk": true, "p2pkh": true, "p2sh": true, "p2tr": false, "p2wpkh": false, "p2wsh": false}
	dbMap           = make(map[string]map[string]int64)
	generalKeyCount = uint64(0)

	miracle = "                   _ooOoo_\n" +
		"                  o8888888o\n" +
		"                  88\" . \"88\n" +
		"                  (| -_- |)\n" +
		"                  O\\  =  /O\n" +
		"               ____/`---'\\____\n" +
		"             .'  \\\\|     |//  `.\n" +
		"            /  \\\\|||  :  |||//  \\\n" +
		"           /  _||||| -:- |||||-  \\\n" +
		"           |   | \\\\\\  -  /// |   |\n" +
		"           | \\_|  ''\\---/''  |   |\n" +
		"           \\  .-\\__  `-`  ___/-. /\n" +
		"         ___`. .'  /--.--\\  `. . __\n" +
		"      .\"\" '<  `.___\\_<|>_/___.'  >'\"\".\n" +
		"     | | :  `- \\`.;`\\ _ /`;.`/ - ` : | |\n" +
		"     \\  \\ `-.   \\_ __\\ /__ _/   .-` /  /\n" +
		"======`-.____`-.___\\_____/___.-`____.-'======\n" +
		"                   `=---='\n" +
		"^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^\n" +
		"         佛祖保佑       财富自由\n"
)

func main() {

	flag.StringVar(&dbDir, "db", "db/", "比特币数据库地址，默认为当前路径的db目录下")
	flag.UintVar(&gcPercent, "gc", 10, "gc参数设置(正整数)（值越小GC越频繁，CPU消耗越高，内存占用少）")
	flag.IntVar(&cpu, "go", 0, "执行业务的携程数量，默认为：cpu*2")
	flag.Parse()
	if cpu == 0 {
		cpu = runtime.NumCPU() * 2
	}

	debug.SetGCPercent(int(gcPercent))
	fmt.Println("-----------------------------begin load db-----------------------------")
	loadDB()
	fmt.Println("-----------------------------over load db-----------------------------")
	for i := 0; i < cpu; i++ {
		go process()
	}

	go func() {
		fmt.Print(miracle)
		var forCount uint64
		for {
			fmt.Printf("\r     已经生成Key数量:%d      ", atomic.LoadUint64(&generalKeyCount))
			time.Sleep(time.Millisecond * 500)
			forCount++
		}
	}()

	c := make(chan os.Signal)
	signal.Notify(c, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	<-c
}

func parseDB(name string) map[string]int64 {
	ret := map[string]int64{}

	files, err := WalkDir(dbDir, name)
	if err != nil {
		panic(err)
	}

	for _, file := range files {
		dbPath := path.Join(dbDir, file)

		f, err := os.Open(dbPath)
		if err != nil {
			return ret
		}

		fmt.Printf("load db file<%s> ......\n", file)

		defer f.Close()
		reader := bufio.NewScanner(f)
		for reader.Scan() {
			datas := strings.Split(reader.Text(), ",")
			if len(datas) != 3 {
				panic(fmt.Sprintf("db<%s> is err!", dbPath))
			}

			amount, err := strconv.ParseInt(datas[0], 10, 64)
			if err != nil {
				fmt.Fprintf(os.Stdout, "parse amount,data is :%s\n", datas[0])
				continue
			}

			ret[datas[2]] = amount
		}
	}

	return ret
}

func WalkDir(dirPth, suffix string) (files []string, err error) {
	files = make([]string, 0, 30)

	err = filepath.Walk(dirPth, func(filename string, fi os.FileInfo, err error) error {
		if fi.IsDir() {
			return nil
		}

		if strings.HasPrefix(fi.Name(), suffix) {
			files = append(files, fi.Name())
		}

		return nil
	})

	return files, err
}

func loadDB() {
	for name, bLoad := range addrMap {
		if bLoad {
			fmt.Printf("load db<%s> ......\n", name)
			dbMap[name] = parseDB(name)
			fmt.Printf("load db<%s> OK !!\n", name)
		}
	}
}

func process() {
	for {
		prvKey, err := btcec.NewPrivateKey(btcec.S256())
		if err != nil {
			fmt.Println("NewPrivateKey err")
			continue
		}

		atomic.AddUint64(&generalKeyCount, 1)
		wif, address, segwitBech32, segwitNested, err := GenerateFromBytes(prvKey, true)
		if err == nil {
			if amount := handle(address, segwitBech32, segwitNested); amount > 0 {
				f, err := os.OpenFile("miracle.text", os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666)
				writeString := fmt.Sprintf("wif:%s,amount:%d,address:%s,compress:true\n", wif, 10, address)
				if err != nil {
					fmt.Println([]byte(writeString))
				} else {
					f.Write([]byte(writeString))
					f.Close()
				}
			}
		}

		//fmt.Printf("------------wif:%s address:%s-----------\n", wif, address)
		wif, address, segwitBech32, segwitNested, err = GenerateFromBytes(prvKey, false)
		if err == nil {
			if amount := handle(address, segwitBech32, segwitNested); amount > 0 {
				f, err := os.OpenFile("miracle.text", os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666)
				writeString := fmt.Sprintf("wif:%s,amount:%d,address:%s compress:false\n", wif, amount, address)
				if err != nil {
					fmt.Println([]byte(writeString))
				} else {
					f.Write([]byte(writeString))
					f.Close()
				}
			}
		}

		time.Sleep(time.Microsecond * 1)
	}

}

func GenerateFromBytes(prvKey *btcec.PrivateKey, compress bool) (wif, address, segwitBech32, segwitNested string, err error) {
	// generate the wif(wallet import format) string
	btcwif, err := btcutil.NewWIF(prvKey, &chaincfg.MainNetParams, compress)
	if err != nil {
		return "", "", "", "", err
	}
	wif = btcwif.String()

	// generate a normal p2pkh address
	serializedPubKey := btcwif.SerializePubKey()
	addressPubKey, err := btcutil.NewAddressPubKey(serializedPubKey, &chaincfg.MainNetParams)
	if err != nil {
		return "", "", "", "", err
	}
	address = addressPubKey.EncodeAddress()

	// generate a normal p2wkh address from the pubkey hash
	witnessProg := btcutil.Hash160(serializedPubKey)
	addressWitnessPubKeyHash, err := btcutil.NewAddressWitnessPubKeyHash(witnessProg, &chaincfg.MainNetParams)
	if err != nil {
		return "", "", "", "", err
	}
	segwitBech32 = addressWitnessPubKeyHash.EncodeAddress()

	// generate an address which is
	// backwards compatible to Bitcoin nodes running 0.6.0 onwards, but
	// allows us to take advantage of segwit's scripting improvments,
	// and malleability fixes.
	serializedScript, err := txscript.PayToAddrScript(addressWitnessPubKeyHash)
	if err != nil {
		return "", "", "", "", err
	}
	addressScriptHash, err := btcutil.NewAddressScriptHash(serializedScript, &chaincfg.MainNetParams)
	if err != nil {
		return "", "", "", "", err
	}
	segwitNested = addressScriptHash.EncodeAddress()

	return wif, address, segwitBech32, segwitNested, nil
}

func handle(address, segwitBech32, segwitNested string) int64 {
	for name, info := range dbMap {
		if name == "p2pk" || name == "p2pkh" {
			amount, ok := info[address]
			if ok {
				return amount
			}
		} else if name == "p2sh" {
			amount, ok := info[segwitNested]
			if ok {
				return amount
			}
		} else if name == "p2wpkh" {
			amount, ok := info[segwitBech32]
			if ok {
				return amount
			}
		}
	}

	return 0
}
