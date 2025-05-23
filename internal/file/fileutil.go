package file

import (
	"github.com/overflow0verture/proxy_harvester/internal/pool"
	"bufio"
	"os"
)

// 将代理池中的代理写入文件
func WriteLinesToFile(proxyStore pool.ProxyStore, lastDataFile string) error {
	proxies, err := proxyStore.GetAll()
	if err != nil {
		return err
	}
	file, err := os.Create(lastDataFile)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	for _, line := range proxies {
		if _, err := writer.WriteString(line + "\n"); err != nil {
			return err
		}
	}

	return writer.Flush()
}

func CreateFolder(folderName string) error {
	if _, err := os.Stat(folderName); os.IsNotExist(err) {
		return os.MkdirAll(folderName, 0755)
	}
	return nil
}

func CreateFile(fileName string) (*os.File, error) {
	return os.Create(fileName)
}


