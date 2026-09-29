package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/jackc/pgx/v5/pgxpool"
)

var C_Pool *pgxpool.Pool

func Connect_Offline() string {
	host := "127.0.0.1"
	port := "5432"
	user := "hacknusa_26"
	password := "secret"
	dbname := "hacknusa_db_26"

	constURL := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		user, password,
		host, port, dbname)
	fmt.Println("Connecting to offline DB:", constURL)

	var err error
	C_Pool, err = pgxpool.New(context.Background(), constURL)
	if err != nil {
		log.Fatalf("Unable to connect to offline database: %v\n", err)
	}
	log.Println("Successfully connected to the offline database")
	return constURL
}
func Connect() string {
	// Relying on AWS secret manager

	secretName := "/prod/hacknusa/db/hacknusa_grading_new"
	region := "ap-southeast-2"

	config, err := config.LoadDefaultConfig(context.Background(), config.WithRegion(region))

	if err != nil {
		log.Fatal(err)
	}
	secretManagerClient := secretsmanager.NewFromConfig(config)

	input := &secretsmanager.GetSecretValueInput{
		SecretId:     aws.String(secretName),
		VersionStage: aws.String("AWSCURRENT"), // VersionStage defaults to AWSCURRENT if unspecified
	}

	result, err := secretManagerClient.GetSecretValue(context.Background(), input)
	if err != nil {
		log.Fatal(err.Error())
	}

	var dbSecret DBSecret
	err = json.Unmarshal([]byte(*result.SecretString), &dbSecret)

	if err != nil {
		log.Fatalf("Error parsing secret JSON: %v", err)
	}

	dbSecret.DBName = "hacknusa_db_26"
	constURL := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=require",
		dbSecret.Username,
		dbSecret.Password,
		dbSecret.Host,
		dbSecret.Port,
		dbSecret.DBName,
	)
	fmt.Println(constURL)
	C_Pool, err = pgxpool.New(context.Background(), constURL)
	if err != nil {
		log.Fatalf("Unable to connect to database: %v\n", err)
	}
	log.Println("Connected to the database")
	return constURL
}
