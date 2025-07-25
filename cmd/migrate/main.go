package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

func main() {
	// Carrega variáveis de ambiente
	if err := godotenv.Load(); err != nil {
		log.Println("Arquivo .env não encontrado, usando variáveis de ambiente do sistema")
	}

	// Conecta ao banco de dados
	db, err := connectDB()
	if err != nil {
		log.Fatalf("Erro ao conectar ao banco de dados: %v", err)
	}
	defer db.Close()

	// Executa as migrações
	if err := runMigrations(db); err != nil {
		log.Fatalf("Erro ao executar migrações: %v", err)
	}

	log.Println("✅ Migrações executadas com sucesso")
}

func connectDB() (*sql.DB, error) {
	host := os.Getenv("DB_HOST")
	port := os.Getenv("DB_PORT")
	user := os.Getenv("DB_USER")
	password := os.Getenv("DB_PASSWORD")
	dbname := os.Getenv("DB_NAME")
	sslmode := os.Getenv("DB_SSLMODE")

	if host == "" {
		host = "localhost"
	}
	if port == "" {
		port = "5432"
	}
	if user == "" {
		user = "txstream_user"
	}
	if password == "" {
		password = "txstream_password"
	}
	if dbname == "" {
		dbname = "txstream_db"
	}
	if sslmode == "" {
		sslmode = "disable"
	}

	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		host, port, user, password, dbname, sslmode)

	return sql.Open("postgres", dsn)
}

func runMigrations(db *sql.DB) error {
	// Lista de migrações em ordem
	migrations := []string{
		"001_create_outbox_table.sql",
		"002_create_orders_table.sql",
		"003_create_events_table.sql",
	}

	for _, migration := range migrations {
		log.Printf("Executando migração: %s", migration)

		// Lê o arquivo de migração
		content, err := os.ReadFile(filepath.Join("migrations", migration))
		if err != nil {
			return fmt.Errorf("erro ao ler arquivo de migração %s: %v", migration, err)
		}

		// Executa a migração
		if _, err := db.Exec(string(content)); err != nil {
			return fmt.Errorf("erro ao executar migração %s: %v", migration, err)
		}

		log.Printf("✅ Migração %s executada com sucesso", migration)
	}

	return nil
}
