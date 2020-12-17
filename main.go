package main

import (
	"context"
	"crypto/tls"
	"database/sql"
	"fmt"
	"net"
	"net/mail"
	"net/smtp"
	"os"
	"strings"
	"time"

	"github.com/lib/pq"
)

const WALMON_ORIGIN = "WALMON_ORIGIN"
const WALMON_DATA_SOURCE_STRING = "WALMON_DATA_SOURCE_STRING"
const WALMON_SMTP_ADDRESS = "WALMON_SMTP_ADDRESS"
const WALMON_SMTP_PORT = "WALMON_SMTP_PORT"
const WALMON_SMTP_USERNAME = "WALMON_SMTP_USERNAME"
const WALMON_SMTP_PASSWORD = "WALMON_SMTP_PASSWORD"
const WALMON_SMTP_DOMAIN = "WALMON_SMTP_DOMAIN"
const WALMON_SMTP_AUTH = "WALMON_SMTP_AUTH"
const WALMON_SMTP_TO = "WALMON_SMTP_TO"
const WALMON_SMTP_FROM = "WALMON_SMTP_FROM"

type SMTPConfig struct {
	Address    string
	Port       string
	User       string
	Password   string
	Domain     string
	AuthMethod string
	From       string
	To         string
}

type Config struct {
	Origin           string
	DataSourceString string
	SMTP             *SMTPConfig
}

func main() {
	config := ReadConfig()

	ok, err := Query(config.DataSourceString)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}

	if ok {
		fmt.Println("WAL archiving status=OK.")
	} else {
		fmt.Println("WAL archiving status=ERROR.")
		err = NotifyError(config)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err)
			os.Exit(1)
		}
	}
}

func ReadConfig() *Config {
	config := &Config{}
	config.Origin = os.Getenv(WALMON_ORIGIN)
	config.DataSourceString = os.Getenv(WALMON_DATA_SOURCE_STRING)

	config.SMTP = &SMTPConfig{}

	config.SMTP.Address = os.Getenv(WALMON_SMTP_ADDRESS)

	config.SMTP.Port = os.Getenv(WALMON_SMTP_PORT)
	if config.SMTP.Port == "" {
		config.SMTP.Port = "587"
	}

	config.SMTP.User = os.Getenv(WALMON_SMTP_USERNAME)
	config.SMTP.Password = os.Getenv(WALMON_SMTP_PASSWORD)
	config.SMTP.Domain = os.Getenv(WALMON_SMTP_DOMAIN)
	config.SMTP.From = os.Getenv(WALMON_SMTP_FROM)
	config.SMTP.To = os.Getenv(WALMON_SMTP_TO)

	config.SMTP.AuthMethod = os.Getenv(WALMON_SMTP_AUTH)
	if config.SMTP.AuthMethod == "" {
		config.SMTP.AuthMethod = "plain"
	}

	return config
}

func Query(dataSourceString string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := sql.Open("postgres", dataSourceString)
	if err != nil {
		return false, err
	}
	defer pool.Close()

	var lastFailedTime pq.NullTime
	var lastArchivedTime pq.NullTime
	err = pool.QueryRowContext(ctx, `SELECT last_failed_time, last_archived_time 
		FROM pg_stat_archiver`).Scan(&lastFailedTime, &lastArchivedTime)
	if err != nil {
		return false, err
	}

	walArchiveFail := lastFailedTime.Valid && ((!lastArchivedTime.Valid) || lastArchivedTime.Time.Before(lastFailedTime.Time))
	return !walArchiveFail, nil
}

func NotifyError(config *Config) error {
	subject := fmt.Sprintf("%s - WAL archiving error", config.Origin)

	var builder strings.Builder
	fmt.Fprintf(&builder, "Origin: %s\r\n", config.Origin)
	fmt.Fprintf(&builder, "Time checked: %s\r\n\r\n", time.Now())

	err := SendEmail(config.SMTP, subject, builder.String())
	return err
}

func SendEmail(config *SMTPConfig, subject string, body string) error {
	var err error
	from := mail.Address{
		Name:    "",
		Address: config.From}
	to := mail.Address{
		Name:    "",
		Address: config.To}

	// Setup headers
	headers := make(map[string]string)
	headers["From"] = from.String()
	headers["To"] = to.String()
	headers["Subject"] = subject

	// Setup message
	message := ""
	for k, v := range headers {
		message += fmt.Sprintf("%s: %s\r\n", k, v)
	}
	message += "\r\n" + body

	// Connect to the SMTP Server
	servername := fmt.Sprintf("%s:%s", config.Address, config.Port)
	host, _, _ := net.SplitHostPort(servername)

	auth := smtp.PlainAuth("", config.User, config.Password, host)

	// TLS config
	tlsconfig := &tls.Config{
		InsecureSkipVerify: false,
		ServerName:         host,
	}

	var client *smtp.Client
	if config.AuthMethod == "plain" {
		client, err = smtp.Dial(servername)
		if err != nil {
			return err
		}

		err = client.StartTLS(tlsconfig)
		if err != nil {
			return err
		}
	} else {
		// Here is the key, you need to call tls.Dial instead of smtp.Dial
		// for smtp servers running on 465 that require an ssl connection
		// from the very beginning (no starttls)
		conn, err := tls.Dial("tcp", servername, tlsconfig)
		if err != nil {
			return err
		}

		client, err = smtp.NewClient(conn, host)
		if err != nil {
			return err
		}
	}

	//domain
	client.Hello(config.Domain)

	// Auth
	if err := client.Auth(auth); err != nil {
		return err
	}

	// To && From
	if err := client.Mail(from.Address); err != nil {
		return err
	}

	if err := client.Rcpt(to.Address); err != nil {
		return err
	}

	// Data
	w, err := client.Data()
	if err != nil {
		return err
	}

	_, err = w.Write([]byte(message))
	if err != nil {
		return err
	}

	err = w.Close()
	if err != nil {
		return err
	}

	client.Quit()
	return nil
}