package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5"
	"github.com/xuri/excelize/v2"
)

type C_ParticipantTeam struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Track string `json:"track"`
}

type C_GradeReq struct {
	AccordanceWTrack     int    `json:"accordance_with_track"`
	UniqueSellingPoint   int    `json:"unique_selling_point"`
	TechnicalFeasibility int    `json:"technical_feasibility"`
	POC                  int    `json:"poc"`
	SecurityPatent       int    `json:"security_patentability"`
	ScalDeploy           int    `json:"scalability_deployability"`
	Comment              string `json:"comment"`
}

type GradeResponse struct {
	ParticipantTeamID    int    `json:"participant_team_id"`
	AccordanceWTrack     int    `json:"accordance_with_track"`
	UniqueSellingPoint   int    `json:"unique_selling_point"`
	TechnicalFeasibility int    `json:"technical_feasibility"`
	POC                  int    `json:"poc"`
	SecurityPatent       int    `json:"security_patentability"`
	ScalDeploy           int    `json:"scalability_deployability"`
	Comment              string `json:"comment"`
}

type ExcelColumn struct {
	JuryId               int
	JuryName             string
	ParticipantTeamName  string
	ParticipantTrack     string
	AccordanceWTrack     int
	UniqueSellingPoint   int
	TechnicalFeasibility int
	POC                  int
	SecurityPatent       int
	TotalScore           int
	ScalDeploy           int
	Comment              string
}

func C_Login(w http.ResponseWriter, r *http.Request) {
	var req LoginReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"Invalid payload"}`, http.StatusBadRequest)
		return
	}

	var juryTeamID int
	var dbPassword string

	fmt.Println(req.Username)
	err := C_Pool.QueryRow(r.Context(), "SELECT jury_team_id, password FROM jury_members WHERE username=$1", req.Username).Scan(&juryTeamID, &dbPassword)
	fmt.Println(juryTeamID, dbPassword)
	if err != nil {
		http.Error(w, `{"error":"Invalid credentials"}`, http.StatusUnauthorized)
		return
	}

	// Simple plain-text string comparison
	if dbPassword != req.Password {
		http.Error(w, `{"error":"Invalid credentials"}`, http.StatusUnauthorized)
		return
	}

	jwtSecret := c_getJWTSecret()
	claims := &Claims{
		JuryTeamID:       juryTeamID,
		RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour))},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, _ := token.SignedString(jwtSecret)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"token": tokenString})
}

func C_GetTeamsHandler(w http.ResponseWriter, r *http.Request) {
	var sql_statement string = "SELECT id, name, track FROM participant_teams ORDER BY id"

	rows, err := C_Pool.Query(r.Context(), sql_statement)
	if err != nil {
		fmt.Println(err)
		http.Error(w, `{"error": "database error"}`, http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var teams []C_ParticipantTeam
	for rows.Next() {
		var team C_ParticipantTeam
		err = rows.Scan(&team.ID, &team.Name, &team.Track)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		teams = append(teams, team)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(teams)
}

func C_GetGradesHandler(w http.ResponseWriter, r *http.Request) {
	participantTeamId, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	juryTeamID := r.Context().Value(juryTeamIDKey).(int)
	//juryTeamID := 1

	var sql_statement string = `SELECT accordance_with_track, unique_selling_point, technical_feasibility, poc, security_patentability, scalability_deployability, comment FROM grades WHERE jury_team_id=$1 AND participant_team_id=$2`
	var pGrade C_GradeReq
	err = C_Pool.QueryRow(r.Context(), sql_statement, juryTeamID, participantTeamId).Scan(&pGrade.AccordanceWTrack, &pGrade.UniqueSellingPoint,
		&pGrade.POC,
		&pGrade.TechnicalFeasibility, &pGrade.SecurityPatent,
		&pGrade.ScalDeploy, &pGrade.Comment)

	if err == pgx.ErrNoRows {
		json.NewEncoder(w).Encode(pGrade)
		return
	} else if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(pGrade)
}

func C_SaveGradesHandler(w http.ResponseWriter, r *http.Request) {
	participantTeamId, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	juryTeamId := r.Context().Value(juryTeamIDKey).(int)
	//juryTeamId := 1

	var req GradeReq
	err = json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if req.AccordanceWTrack < 0 || req.AccordanceWTrack > 5 ||
		req.UniqueSellingPoint < 0 || req.UniqueSellingPoint > 5 ||
		req.TechnicalFeasibility < 0 || req.TechnicalFeasibility > 5 ||
		req.POC < 0 || req.POC > 5 ||
		req.SecurityPatent < 0 || req.SecurityPatent > 5 ||
		req.ScalDeploy < 0 || req.ScalDeploy > 5 {
		http.Error(w, `{"error":"Scores must be between 0 and 5"}`, http.StatusBadRequest)
		return
	}

	var sql_statement string = `INSERT INTO grades (jury_team_id, participant_team_id, accordance_with_track, unique_selling_point, technical_feasibility, poc, security_patentability, scalability_deployability, comment)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	ON CONFLICT(jury_team_id, participant_team_id) DO UPDATE SET
		accordance_with_track = EXCLUDED.accordance_with_track,
		unique_selling_point = EXCLUDED.unique_selling_point,
		technical_feasibility = EXCLUDED.technical_feasibility,
		poc = EXCLUDED.poc,
		security_patentability = EXCLUDED.security_patentability,
		scalability_deployability = EXCLUDED.scalability_deployability,

		comment = EXCLUDED.comment`
	_, err = C_Pool.Exec(r.Context(), sql_statement, juryTeamId,
		participantTeamId, req.AccordanceWTrack,
		req.UniqueSellingPoint, req.TechnicalFeasibility, req.POC,
		req.SecurityPatent, req.ScalDeploy, req.Comment)

	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error ": "%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

func C_GetAllGradesJury(w http.ResponseWriter, r *http.Request) {
	juryTeamId, ok := r.Context().Value(juryTeamIDKey).(int)
	if !ok {
		http.Error(w, `{"error": "Unauthorized"}`, http.StatusInternalServerError)
		return
	}

	var sql_statement string = "SELECT participant_team_id, accordance_with_track, unique_selling_point, technical_feasibility, poc, security_patentability, scalability_deployability, comment FROM grades WHERE jury_team_id=$1"

	rows, err := C_Pool.Query(r.Context(), sql_statement, juryTeamId)
	if err != nil {
		http.Error(w, `{"error": "Database error"}`, http.StatusInternalServerError)
		return
	}

	defer rows.Close()
	var grades []GradeResponse
	for rows.Next() {
		var grade GradeResponse
		err = rows.Scan(
			&grade.ParticipantTeamID,
			&grade.AccordanceWTrack,
			&grade.UniqueSellingPoint,
			&grade.TechnicalFeasibility,
			&grade.POC,
			&grade.SecurityPatent,
			&grade.ScalDeploy,
			&grade.Comment)
		if err != nil {
			http.Error(w, `{"error": "Scan Error"}`, http.StatusInternalServerError)
			return
		}
		grades = append(grades, grade)
	}

	if grades == nil {
		grades = []GradeResponse{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(grades)
}

func ExportGradesHandler(w http.ResponseWriter, r *http.Request) {
	sqlStatement := `
		SELECT 
			jt.id as jury_id,
			jt.name as jury_team_name,
			pt.name as participant_team_name,
			pt.track as participant_track,
			g.accordance_with_track,
			g.unique_selling_point,
			g.technical_feasibility,
			g.poc,
			g.security_patentability,
			g.scalability_deployability,
			(g.accordance_with_track + g.unique_selling_point + g.technical_feasibility + g.poc + g.security_patentability + g.scalability_deployability) as Total_Score,
			COALESCE(g.comment, '')

			FROM grades g
			JOIN jury_teams jt on jt.id = g.jury_team_id
			JOIN participant_teams pt on pt.id = g.participant_team_id
			ORDER BY pt.id, jt.id
	`

	rows, err := C_Pool.Query(r.Context(), sqlStatement)
	if err != nil {
		fmt.Println("ERROR", err)
		return
	}
	f_e := excelize.NewFile()
	var sheetName string = "Grades Export"
	index, err := f_e.NewSheet(sheetName)
	f_e.SetActiveSheet(index)

	var e_headers []string = []string{
		"jury_id", "jury_name", "participant_team", "participant_track",
		"accorance_with_track", "unique_selling_point", "techinical_feasibility",
		"poc", "security_patentability", "scalability_deployability",
		"total_scores", "comment",
	}

	for colIdx, header := range e_headers {
		cell, _ := excelize.CoordinatesToCellName(colIdx+1, 1)
		f_e.SetCellValue(sheetName, cell, header)
	}

	rowsIdx := 2
	for rows.Next() {
		var cell_data ExcelColumn
		fmt.Println(rows.Values())
		err := rows.Scan(&cell_data.JuryId, &cell_data.JuryName,
			&cell_data.ParticipantTeamName, &cell_data.ParticipantTrack, &cell_data.AccordanceWTrack,
			&cell_data.UniqueSellingPoint, &cell_data.TechnicalFeasibility,
			&cell_data.POC, &cell_data.SecurityPatent, &cell_data.ScalDeploy,
			&cell_data.TotalScore, &cell_data.Comment)
		if err != nil {
			msg := fmt.Sprintf(`{"error": "Failed to scan the database"} %s`, err)
			http.Error(w, msg, http.StatusInternalServerError)
			return
		}

		inject_data := []interface{}{
			cell_data.JuryId,
			cell_data.JuryName,
			cell_data.ParticipantTeamName,
			cell_data.ParticipantTrack,
			cell_data.AccordanceWTrack,
			cell_data.UniqueSellingPoint,
			cell_data.TechnicalFeasibility,
			cell_data.POC,
			cell_data.SecurityPatent,
			cell_data.ScalDeploy,
			cell_data.TotalScore,
			cell_data.Comment,
		}

		for colIdx, value := range inject_data {
			cell, _ := excelize.CoordinatesToCellName(colIdx+1, rowsIdx)
			f_e.SetCellValue(sheetName, cell, value)
		}
		rowsIdx += 1
	}

	filename := fmt.Sprintf("Hackathon_Grades_%s.xlsx", time.Now().Format("2006-01-02"))
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	err = f_e.Write(w)
	if err != nil {
		http.Error(w, `{"error":"Failed to write Excel file"}`, http.StatusInternalServerError)
		return
	}

}
