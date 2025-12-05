package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

type UserRequest struct {
	Message string `json:"message"`
}

type SearchQuery struct {
	Name     string `json:"name,omitempty"`
	Location string `json:"location,omitempty"`
	Format   string `json:"format,omitempty"`
	Опыт     string `json:"опыт,omitempty"`
}

// Ответ пользователю — либо список вакансий, либо текст от LLM
type GatewayResponse struct {
	Type      string        `json:"type"` // "vacancies" или "answer"
	Vacancies []interface{} `json:"vacancies,omitempty"` // сырые JSON-объекты из search_service
	Answer    string        `json:"answer,omitempty"`
}

type LLMOutput struct {
	Status       string `json:"status"`
	Data         struct {
		Name     string `json:"name,omitempty"`
		Location string `json:"location,omitempty"`
		Опыт     string `json:"опыт,omitempty"`
	} `json:"data"`
	ErrorMessage string `json:"error_message,omitempty"`
}

type LLMWrapperResponse struct {
	Response string `json:"response"`
}

var (
	httpClient = &http.Client{
		Timeout: 15 * time.Second,
	}
	searchServiceURL string
	llmWrapperURL    string
)

// CORS middleware для открытия доступа со всех доменов
func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Разрешить запросы с любого origin
		w.Header().Set("Access-Control-Allow-Origin", "*")
		// Разрешить только POST и OPTIONS
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		// Разрешить заголовок Content-Type
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		// Обработка preflight-запроса
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next(w, r)
	}
}

func main() {
	llmWrapperURL = os.Getenv("LLM_WRAPPER_URL")
	if llmWrapperURL == "" {
		llmWrapperURL = "http://llm_wrapper:8080/"
	}

	searchServiceURL = os.Getenv("SEARCH_SERVICE_URL")
	if searchServiceURL == "" {
		searchServiceURL = "http://search_service:8083/search"
	}



		var req UserRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON: expected {\"message\": \"...\"}", http.StatusBadRequest)
			return
		}

		if req.Message == "" {
			http.Error(w, "'message' is required", http.StatusBadRequest)
			return
		}

		prompt := fmt.Sprintf(`Ты — строгий парсер запросов на поиск IT-вакансий.
Твоя задача — проанализировать сообщение и вывести ТОЛЬКО валидный JSON, без текста до или после.

Формат ответа:
{"status":"success","data":{"name":"...","location":"...", format:"", "опыт":"not specified|Нет опыта|"От 1 года до 3 лет|"От 3 до 6 лет|Более 6 лет|"}}  
или  
{"status":"error","error_message":"..."}

Правила:
1. Если в запросе есть: должность (python, java, devops), город (Москва, Минск), или слова "вакансия", "работа", "найти" — ставь "status":"success", извлеки данные.
2. Иначе — "status":"error", с сообщением.

ТОЛЬКО JSON. Никаких пояснений.

Запрос пользователя: "%s"`, req.Message)

		llmReq := map[string]string{"message": prompt}
		llmReqBody, _ := json.Marshal(llmReq)

		resp, err := httpClient.Post(llmWrapperURL, "application/json", bytes.NewBuffer(llmReqBody))
		if err != nil {
			log.Printf("LLM call failed (%s): %v", llmWrapperURL, err)
			http.Error(w, "AI service unavailable", http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			log.Printf("LLM returned %d", resp.StatusCode)
			http.Error(w, "AI processing failed", http.StatusInternalServerError)
			return
		}

		llmRespBody, err := io.ReadAll(resp.Body)
		if err != nil {
			http.Error(w, "Failed to read AI response", http.StatusInternalServerError)
			return
		}

		var outer LLMWrapperResponse
		if err := json.Unmarshal(llmRespBody, &outer); err != nil {
			log.Printf("Failed to parse outer response: %s", string(llmRespBody))
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(GatewayResponse{
				Type:   "answer",
				Answer: "Ошибка формата от AI-сервиса.",
			})
			return
		}

		var llmOut LLMOutput
		if err := json.Unmarshal([]byte(outer.Response), &llmOut); err != nil {
			log.Printf("Failed to parse inner JSON: %s", outer.Response)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(GatewayResponse{
				Type:   "answer",
				Answer: "AI вернул некорректный JSON. Попробуйте уточнить запрос.",
			})
			return
		}

		if llmOut.Status == "success" {
			// ✅ Собираем параметры и отправляем в search_service
			searchQuery := SearchQuery{
				Name:     llmOut.Data.Name,
				Location: llmOut.Data.Location,
				Опыт:     llmOut.Data.Опыт,
			}

			searchBody, _ := json.Marshal(searchQuery)
			searchResp, err := httpClient.Post(searchServiceURL, "application/json", bytes.NewBuffer(searchBody))
			if err != nil {
				log.Printf("Search service call failed: %v", err)
				http.Error(w, "Search service unavailable", http.StatusBadGateway)
				return
			}
			defer searchResp.Body.Close()

			searchResult, err := io.ReadAll(searchResp.Body)
			if err != nil || searchResp.StatusCode != http.StatusOK {
				log.Printf("Search service error: %d, body: %s", searchResp.StatusCode, string(searchResult))
				http.Error(w, "Search failed", http.StatusInternalServerError)
				return
			}

			// Парсим как []interface{} чтобы просто вернуть JSON как есть
			var vacancies []interface{}
			if err := json.Unmarshal(searchResult, &vacancies); err != nil {
				log.Printf("Failed to parse vacancies: %s", string(searchResult))
				http.Error(w, "Invalid search response", http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(GatewayResponse{
				Type:      "vacancies",
				Vacancies: vacancies,
			})

		} else {
			// ❌ Простой ответ от LLM
			msg := llmOut.ErrorMessage
			if msg == "" {
				msg = "Запрос не распознан как поиск вакансий."
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(GatewayResponse{
				Type:   "answer",
				Answer: msg,
			})
		}
	}))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8082"
	}

	log.Printf("Prompt Gateway running on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}