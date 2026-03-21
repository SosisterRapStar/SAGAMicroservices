package integration

import (
	"fmt"
	"strings"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	Services   Services   `yaml:"services"`
	BookingDB  DBConfig   `yaml:"booking_db"`
	FlightsDB  DBConfig   `yaml:"flights_db"`
	HotelsDB   DBConfig   `yaml:"hotels_db"`
}

type Services struct {
	Booking ServiceAddr `yaml:"booking"`
	Flights ServiceAddr `yaml:"flights"`
	Hotels  ServiceAddr `yaml:"hotels"`
}

type ServiceAddr struct {
	Address string `yaml:"address"`
}

type DBConfig struct {
	Hosts    string `yaml:"hosts"`
	Dbname   string `yaml:"dbname"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	Schema   string `yaml:"schema"`
}

func (d *DBConfig) PostgresDSN() string {
	parts := strings.Split(d.Hosts, ":")
	host, port := parts[0], parts[1]
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		host, port, d.User, d.Password, d.Dbname,
	)
	if d.Schema != "" {
		dsn += fmt.Sprintf(" search_path=%s", d.Schema)
	}
	return dsn
}

func (d *DBConfig) MySQLDSN() string {
	return fmt.Sprintf(
		"%s:%s@tcp(%s)/%s?parseTime=true&charset=utf8mb4",
		d.User, d.Password, d.Hosts, d.Dbname,
	)
}

func loadConfig(path string) (*Config, error) {
	var cfg Config
	if err := cleanenv.ReadConfig(path, &cfg); err != nil {
		return nil, fmt.Errorf("reading integration config %s: %w", path, err)
	}
	return &cfg, nil
}
