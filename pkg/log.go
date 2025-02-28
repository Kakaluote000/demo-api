package pkg

import (
	"os"

	"github.com/sirupsen/logrus"
)

var Log *logrus.Logger

func init() {
	Log = InitLogrus()
	Log.Info("Logger initialized")
}

func InitLogrus() *logrus.Logger {
	log := logrus.New()

	// 创建日志文件
	file, err := os.OpenFile("app.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("Failed to log to file, using default stderr")
	}

	// 设置日志输出到文件
	log.SetOutput(file)

	// 设置日志格式
	log.SetFormatter(&logrus.JSONFormatter{})

	return log
}
