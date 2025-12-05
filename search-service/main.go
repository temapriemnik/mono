// main.go
package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"

	_ "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/go-chi/chi/v5"
)

type Vacancy struct {
	ID                  int32  `json:"id"`
	City                string `json:"city"`
	Name                string `json:"name"`
	RequiredExperience  string `json:"required_experience"`
	Description         string `json:"description"`
	SalaryFrom          int32  `json:"salary_from"`
	SalaryTo            int32  `json:"salary_to"`
	PublishedAt         string `json:"published_at"`
	Status              string `json:"status"`
	Skills              string `json:"skills"`
}

type Synonyms map[string][]string

var (
	synonyms Synonyms
	cityRe   = regexp.MustCompile(`(?i)\b(москв[ауе]|санкт-петербург|минск|екатеринбург|новосибирск|казань|самара|ростов|владивосток|челябинск|нижний новгород|уфа|краснодар)\b`)
)

func loadSynonyms(path string) (Synonyms, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s Synonyms
	err = json.Unmarshal(data, &s)
	return s, err
}

func normalizeExperience(query string) string {
	q := strings.ToLower(query)
	if regexp.MustCompile(`(?i)\b(без\s*опыт|стажер|джун|junior|начинающий|0\s*лет)\b`).MatchString(q) {
		return "Нет опыта"
	}
	if regexp.MustCompile(`(?i)\b(1\s*-\s*3|от\s*1\s*до\s*3|мидл|middle|1\s*год|2\s*года|3\s*года)\b`).MatchString(q) {
		return "От года до трех лет"
	}
	if regexp.MustCompile(`(?i)\b(3\s*-\s*6|от\s*3\s*до\s*6|сеньор|senior|4\s*года|5\s*лет|6\s*лет)\b`).MatchString(q) {
		return "От 3 до 6 лет"
	}
	if regexp.MustCompile(`(?i)\b(более\s*6|6\+\s*лет|lead|техлид|архитектор)\b`).MatchString(q) {
		return "Более 6 лет"
	}
	return ""
}

func extractKeyword(query string, syns Synonyms) string {
	q := strings.ToLower(query)
	for canonical, variants := range syns {
		for _, v := range variants {
			if strings.Contains(q, v) {
				return canonical
			}
		}
	}
	return ""
}

func parseQuery(q string) (city, keyword, expHH string) {
	if matches := cityRe.FindStringSubmatch(q); len(matches) > 0 {
		city = strings.Title(strings.ToLower(matches[0]))
		if strings.HasPrefix(strings.ToLower(city), "москв") {
			city = "Москва"
		} else if strings.Contains(strings.ToLower(city), "санкт") {
			city = "Санкт-Петербург"
		}
		// Можно расширить
	}
	keyword = extractKeyword(q, synonyms)
	expHH = normalizeExperience(q)
	return
}

func buildWhere(city, keyword, expHH string) (string, []interface{}) {
	where := []string{"1 = 1"}
	args := []interface{}{}

	if city != "" {
		where = append(where, "city = ?")
		args = append(args, city)
	}
	if keyword != "" {
		kw := "%" + keyword + "%"
		where = append(where, "(LOWER(name) LIKE ? OR LOWER(skills) LIKE ? OR LOWER(description) LIKE ?)")
		args = append(args, kw, kw, kw)
	}
	if expHH != "" {
		where = append(where, "required_experience = ?")
		args = append(args, expHH)
	}
	return strings.Join(where, " AND "), args
}

func trySearch(db *sql.DB, city, keyword, expHH string, offset, limit int) ([]Vacancy, string) {
	where, args := buildWhere(city, keyword, expHH)
	sqlStr := fmt.Sprintf(`
		SELECT id, city, name, required_experience, description, salary_from, salary_to,
		       toString(published_at) as published_at, status, skills
		FROM vacancies
		WHERE %s
		ORDER BY published_at DESC
		LIMIT %d OFFSET %d`,
		where, limit, offset)

	rows, err := db.Query(sqlStr, args...)
	if err != nil {
		log.Printf("SQL ERROR: %v | Query: %s | Args: %v", err, sqlStr, args)
		return nil, sqlStr
	}
	defer rows.Close()

	var results []Vacancy
	for rows.Next() {
		var v Vacancy
		if err := rows.Scan(&v.ID, &v.City, &v.Name, &v.RequiredExperience, &v.Description,
			&v.SalaryFrom, &v.SalaryTo, &v.PublishedAt, &v.Status, &v.Skills); err != nil {
			continue
		}
		results = append(results, v)
	}
	return results, sqlStr
}

func searchHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("q")
		pageStr := r.URL.Query().Get("page")
		page := 0
		if pageStr != "" {
			if p, err := strconv.Atoi(pageStr); err == nil && p >= 0 {
				page = p
			}
		}
		offset := page * 10
		limit := 10

		if query == "" {
			http.Error(w, "missing 'q' param", 400)
			return
		}

		city, keyword, expHH := parseQuery(query)
		log.Printf("Parsed: city=%s, keyword=%s, experience=%s, page=%d", city, keyword, expHH, page)

		var results []Vacancy
		var lastSQL string

		// Попытка 1: все фильтры
		results, lastSQL = trySearch(db, city, keyword, expHH, offset, limit)
		if len(results) == limit {
			goto respond
		}

		// Попытка 2: без опыта
		if expHH != "" {
			results, lastSQL = trySearch(db, city, keyword, "", offset, limit)
			if len(results) == limit {
				goto respond
			}
		}

		// Попытка 3: без города
		if city != "" {
			results, lastSQL = trySearch(db, "", keyword, expHH, offset, limit)
			if len(results) == limit {
				goto respond
			}
		}

		// Попытка 4: только ключевое слово
		if keyword != "" {
			results, lastSQL = trySearch(db, "", keyword, "", offset, limit)
			if len(results) == limit {
				goto respond
			}
		}

		// Попытка 5: без фильтров (любые вакансии)
		results, lastSQL = trySearch(db, "", "", "", offset, limit)

	respond:
		log.Printf("Final SQL: %s → %d results", lastSQL, len(results))
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		json.NewEncoder(w).Encode(results)
	}
}

func main() {
	var err error
	synonyms, err = loadSynonyms("config/synonyms.json")
	if err != nil {
		log.Fatalf("Failed to load synonyms from config/synonyms.json: %v", err)
	}

	// Подключение к ClickHouse (пароль: password)
	conn, err := sql.Open("clickhouse", "clickhouse://default:password@localhost:9000/default")
	if err != nil {
		log.Fatal("Failed to open ClickHouse connection:", err)
	}
	defer conn.Close()

	if err := conn.Ping(); err != nil {
		log.Fatal("ClickHouse ping failed:", err)
	}

	r := chi.NewRouter()
	r.Get("/search", searchHandler(conn))

	log.Println("✅ Search service started on http://localhost:8080")
	log.Println("💡 Example: http://localhost:8080/search?q=джун питон в Москве&page=0")
	log.Fatal(http.ListenAndServe(":8080", r))
}