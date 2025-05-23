package globals

var (
	//SocksList     []string
	//EffectiveList []string
	//ProxyIndex    int
	//Timeout       int
	//LastDataFile  = "lastData.txt"
	//Wg            sync.WaitGroup
	//Mu            sync.Mutex
	//Semaphore     chan struct{}
	ToCheckChan chan string
)

// 获取当前代理索引
//func GetCurrentProxyIndex() int {
//	Mu.Lock()
//	defer Mu.Unlock()
//	return ProxyIndex
//}

// 设置下一个代理索引
//func SetNextProxyIndex() {
//	Mu.Lock()
//	defer Mu.Unlock()
//	if len(EffectiveList) > 0 {
//		ProxyIndex = (ProxyIndex + 1) % len(EffectiveList)
//	}
//}

// InitFetchChannel 初始化待检测通道
func InitFetchChannel(size int) {
	ToCheckChan = make(chan string, size)
}
