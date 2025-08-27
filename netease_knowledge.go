package services

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

type NetEaseKnowledgeService struct {
	client *http.Client
	Token  string
}

func NewNetEaseKnowledgeService() *NetEaseKnowledgeService {
	return &NetEaseKnowledgeService{
		client: &http.Client{},
	}
}

func (s *NetEaseKnowledgeService) Login() error {
	requestBody := map[string]any{
		"cmd":         "kefu_get_token",
		"uid":         "", //改为你自己的
		"game_uid":    "", //改为你自己的
		"os":          "android",
		"return_buff": false,
	}

	jsonData, jsonDataErr := json.Marshal(requestBody)
	if jsonDataErr != nil {
		fmt.Printf("JSON编码错误: %v", jsonDataErr)
		return jsonDataErr
	}

	req, reqErr := s.client.Post("https://live-gms-sky-merge.game.163.com:9005/gms_cmd", "application/json", bytes.NewBuffer(jsonData))
	if reqErr != nil {
		fmt.Printf("请求发送失败: %v", reqErr)
		return reqErr
	}
	defer req.Body.Close()

	body, _ := io.ReadAll(req.Body)

	//检查响应码
	if req.StatusCode != http.StatusOK {
		fmt.Printf("响应状态码错误: %d", req.StatusCode)
		fmt.Printf("响应内容: %s", body)
		return errors.New("响应状态码错误")
	}

	//解析响应
	var response map[string]any
	if err := json.Unmarshal(body, &response); err != nil {
		fmt.Printf("JSON解析错误: %v", err)
		return err
	}

	// 检查status是否为ok
	if status, ok := response["status"].(string); !ok || status != "ok" {
		fmt.Printf("响应状态错误: %v", response)
		return errors.New("响应状态不是ok")
	}

	// 提取result字段并解析其中的token
	if resultStr, ok := response["result"].(string); ok {
		var result map[string]any
		if err := json.Unmarshal([]byte(resultStr), &result); err != nil {
			fmt.Printf("result字段JSON解析错误: %v", err)
			return err
		}

		// 提取token并存入全局Token字段
		if token, ok := result["token"].(string); ok {
			s.Token = token
			return nil
		} else {
			fmt.Printf("token字段不存在或不是字符串: %v", result)
			return errors.New("token字段不存在")
		}
	} else {
		fmt.Printf("result字段不存在或不是字符串: %v", response)
		return errors.New("result字段不存在")
	}
}

func (s *NetEaseKnowledgeService) GetKnowledgeAnswer(question string) (string, error) {
	// 第一次尝试获取答案
	answer, err := s.getKnowledgeAnswerInternal(question)
	if err == nil {
		return answer, nil
	}

	// 检查是否为未登录错误
	if err.Error() == "未登录" {
		// 尝试登录
		if loginErr := s.Login(); loginErr != nil {
			return "", fmt.Errorf("登录失败: %v", loginErr)
		}

		// 登录成功后重试获取答案
		answer, err = s.getKnowledgeAnswerInternal(question)
		if err != nil {
			return "", fmt.Errorf("重试请求失败: %v", err)
		}
		return answer, nil
	}

	// 其他错误直接返回
	return "", err
}

func (s *NetEaseKnowledgeService) getKnowledgeAnswerInternal(question string) (string, error) {
	// 构建请求体
	requestBody := map[string]any{
		"question": question,
	}

	jsonData, jsonDataErr := json.Marshal(requestBody)
	if jsonDataErr != nil {
		return "", fmt.Errorf("JSON编码错误: %v", jsonDataErr)
	}

	// 创建请求
	req, reqErr := http.NewRequest("POST", "https://sprite.gameyw.netease.com/sprite/api/ma75/knowledge/get", bytes.NewBuffer(jsonData))
	if reqErr != nil {
		return "", fmt.Errorf("请求创建失败: %v", reqErr)
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("token", s.Token)
	req.Header.Set("token-type", "gmsdk")

	// 发送请求
	resp, respErr := s.client.Do(req)
	if respErr != nil {
		return "", fmt.Errorf("请求发送失败: %v", respErr)
	}
	defer resp.Body.Close()

	// 读取响应体
	body, bodyErr := io.ReadAll(resp.Body)
	if bodyErr != nil {
		return "", fmt.Errorf("响应读取失败: %v", bodyErr)
	}

	// 检查响应状态码
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("响应状态码错误: %d, 响应内容: %s", resp.StatusCode, string(body))
	}

	// 解析响应
	var response map[string]any
	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("JSON解析错误: %v", err)
	}

	// 检查响应码
	if code, ok := response["code"].(float64); ok {
		switch code {
		case 200:
			// 正常响应，提取answer字段
			if data, ok := response["data"].(map[string]any); ok {
				if answer, ok := data["answer"].(string); ok {
					return answer, nil
				}
				return "", errors.New("answer字段不存在或不是字符串")
			}
			return "", errors.New("data字段不存在或不是对象")

		case 4001:
			// 未登录错误
			return "", errors.New("未登录")

		default:
			// 其他错误码
			if message, ok := response["message"].(string); ok {
				return "", fmt.Errorf("API错误: %s", message)
			}
			return "", errors.New("未知API错误")
		}
	}

	return "", errors.New("code字段不存在或不是数字")
}
