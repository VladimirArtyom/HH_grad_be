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

const accordance string = "accordance_with_track"
const usp string = "unique_selling_point"
const poc string = "poc"
const security string = "security_patentability"
const technical string = "technical_feasibility"
const scalability string = "scalability_deployability"

var roleAllowedFields = map[string][]string{
	"admin":              {accordance, usp, poc, security, technical, scalability},
	"solution_architect": {accordance, usp, poc, security, technical, scalability},
	"business":           {accordance, usp, scalability},
	"tech_security":      {accordance, usp, poc, security, technical},
	"tech_hcl":           {accordance, usp, poc, security, technical},
}

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
		fmt.Println(err)
		http.Error(w, `{"error":"Invalid payload"} `, http.StatusBadRequest)
		return
	}

	var juryTeamID int
	var dbPassword string
	var role string
	var juryName string

	const sql_query = "select jt.id, jm.password, jt.role, jm.name from jury_members as jm INNER JOIN jury_teams jt on jm.id=jt.id WHERE jm.name=$1"
	err := C_Pool.QueryRow(r.Context(), sql_query, req.Username).Scan(&juryTeamID, &dbPassword, &role, &juryName)
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
	claims := &C_Claims{
		JuryTeamID:       juryTeamID,
		Role:             role,
		RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour))},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, _ := token.SignedString(jwtSecret)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"token": tokenString, "role": role})
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
	fmt.Println(juryTeamID)
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
	juryRole := r.Context().Value(roleKey).(string)
	//juryTeamId := 1
	var req C_GradeReq
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
	allowedFields, exists := roleAllowedFields[juryRole]
	isAllowed := func(field string) bool {
		for _, f := range allowedFields {
			if f == field {
				return true
			}
		}
		return false
	}

	if !exists {
		http.Error(w, `{"error": "Forbidden: Invalid role"}`, http.StatusForbidden)
		return
	}

	var existingGrade C_GradeReq
	var existingComment string
	var sql_existing_statement string = `SELECT accordance_with_track, unique_selling_point, technical_feasibility, poc, security_patentability, scalability_deployability, comment 
		FROM grades where jury_team_id=$1 AND participant_team_id=$2
	`
	err = C_Pool.QueryRow(r.Context(), sql_existing_statement, juryTeamId, participantTeamId).Scan(
		&existingGrade.AccordanceWTrack, &existingGrade.UniqueSellingPoint,
		&existingGrade.TechnicalFeasibility, &existingGrade.POC,
		&existingGrade.SecurityPatent, &existingGrade.ScalDeploy,
		&existingComment,
	)
	if err != nil && err != pgx.ErrNoRows {
		fmt.Printf("Error %v", err)
		http.Error(w, `{"error" : "Database error"}`, http.StatusInternalServerError)
		return
	}

	if isAllowed(accordance) {
		existingGrade.AccordanceWTrack = req.AccordanceWTrack
	}

	if isAllowed(poc) {
		existingGrade.POC = req.POC
	}

	if isAllowed(scalability) {
		existingGrade.ScalDeploy = req.ScalDeploy
	}

	if isAllowed(security) {
		existingGrade.SecurityPatent = req.SecurityPatent
	}

	if isAllowed(usp) {
		existingGrade.UniqueSellingPoint = req.UniqueSellingPoint
	}

	if isAllowed(technical) {
		existingGrade.TechnicalFeasibility = req.TechnicalFeasibility
	}

	if req.Comment != "" {
		existingComment = req.Comment
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
		participantTeamId, existingGrade.AccordanceWTrack,
		existingGrade.UniqueSellingPoint, existingGrade.TechnicalFeasibility,
		existingGrade.POC, existingGrade.SecurityPatent, existingGrade.ScalDeploy, existingComment)

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
