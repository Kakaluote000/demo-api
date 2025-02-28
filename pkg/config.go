package pkg

import (
	"github.com/spf13/viper"
)

// InitConfig 初始化配置
func InitConfig() {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./config")
	if err := viper.ReadInConfig(); err != nil {
		Log.Fatalf("Error reading config file: %v", err)
	}
}
