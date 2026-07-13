package tool

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/spf13/viper"
	"github.com/zaigie/palworld-server-tool/internal/executor"
	"github.com/zaigie/palworld-server-tool/internal/logger"
)

func CustomCommand(command string) (string, error) {
	if strings.TrimSpace(viper.GetString("rcon.address")) == "" {
		return "", fmt.Errorf("rcon address is empty")
	}
	exec, err := executor.NewExecutor(
		viper.GetString("rcon.address"),
		viper.GetString("rcon.password"),
		viper.GetInt("rcon.timeout"),
		true,
	)
	if err != nil {
		return "", err
	}
	defer exec.Close()

	useBase64 := viper.GetBool("rcon.use_base64")
	if useBase64 {
		command = base64.StdEncoding.EncodeToString([]byte(command))
	}
	response, err := exec.Execute(command)
	if err != nil {
		return "", err
	}
	if !useBase64 {
		return response, nil
	}
	decoded, decodeErr := base64.StdEncoding.DecodeString(response)
	if decodeErr != nil {
		logger.Warnf("decode RCON base64 response error: %v\n", decodeErr)
		return response, nil
	}
	return string(decoded), nil
}
