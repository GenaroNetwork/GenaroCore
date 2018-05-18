package state
import(
	"math/rand"
	"testing"
	"time"
)


var nowblock uint64
var nownum uint64

func init(){
	nowblock = 0
	nownum = 0
}

func GenNumLog () NumLog {
	rand.Seed(int64(time.Now().Nanosecond()))
	nowblock += uint64(rand.Intn(10))
	nownum += uint64(rand.Intn(10))

	log :=NumLog{nowblock,nownum}
	return log
}

func display(t *testing.T,logs NumLogs){
	for _,log := range logs {
		t.Log(log)
	}
}

func TestAddLog(t *testing.T){
	logs := new(NumLogs)
	for i:=0;i<100;i++{
		logs.add(GenNumLog ())
	}
	display(t ,*logs )
}

func TestFindFirst(t *testing.T) {
	logs := new(NumLogs)
	for i:=0;i<1000000;i++{
		//t.Log(i)
		log:=GenNumLog ()
		//t.Log(log)
		logs.add(log)
	}
	t.Log()
	//start := time.Now()
	t.Log(time.Now())
	log,id := logs.GetFirstAfterBlockNum(3456000,0,uint64(len(*logs) -1))
	//t.Log(time.Since(start))
	t.Log(time.Now())
	t.Log(id,log)
}

func TestFindLast(t *testing.T) {
	logs := new(NumLogs)
	for i:=0;i<1000;i++{
		t.Log(i)
		log:=GenNumLog ()
		t.Log(log)
		logs.add(log)
	}
	t.Log()
	//start := time.Now()
	t.Log(time.Now())
	log,id := logs.GetLastBeforBlockNum(3056,0,uint64(len(*logs) -1))
	//t.Log(time.Since(start))
	t.Log(time.Now())
	t.Log(id,log)
}