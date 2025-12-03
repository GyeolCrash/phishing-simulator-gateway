package storage

import (
	"PishingSimulator_SecurityProject/internal/models"
	"strings"
	"time"
)

func CreateRecords(userID int, scenarioKey string, filePath string) error {
	stmt, err := db.Prepare("INSERT INTO records(user_id, scenario_key, file_path, created_at) VALUES(?, ?, ?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(userID, scenarioKey, filePath, time.Now())
	return err
}

func GetRecordsByUserID(userID int) ([]models.Record, error) {
	query := `
        SELECT id, scenario_key, file_path, created_at 
        FROM records 
        WHERE user_id = ? 
        ORDER BY created_at DESC
    `
	rows, err := db.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []models.Record
	for rows.Next() {
		var r models.Record
		var createdStr string

		if err := rows.Scan(&r.ID, &r.Scenario, &r.FilePath, &createdStr); err != nil {
			return nil, err
		}

		// [핵심 수정 1] "m=+..." (Monotonic Clock) 부분 제거
		// DB에 저장된 문자열: "2025-11-19 12:04:18.416585929 +0000 UTC m=+202.123..."
		// 파싱을 위해 " m=" 뒷부분을 잘라냅니다.
		if idx := strings.Index(createdStr, " m="); idx != -1 {
			createdStr = createdStr[:idx]
		}

		// [핵심 수정 2] Go 기본 문자열 포맷 레이아웃 적용
		// "2025-11-19 12:04:18.416585929 +0000 UTC" 형식을 읽기 위한 레이아웃
		layout := "2006-01-02 15:04:05.999999999 -0700 MST"

		parsedTime, err := time.Parse(layout, createdStr)
		if err != nil {
			// 혹시라도 실패하면 RFC3339 등 다른 시도 (기존 데이터 호환성)
			parsedTime, err = time.Parse(time.RFC3339, createdStr)
			if err != nil {
				// 정 안되면 현재 시간이라도 넣어서 에러 방지
				r.CreatedAt = time.Time{}
			} else {
				r.CreatedAt = parsedTime
			}
		} else {
			r.CreatedAt = parsedTime
		}

		records = append(records, r)
	}
	return records, nil
}
