package integration

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

const (
	testUserID   = "11111111-1111-1111-1111-111111111111"
	testFlightID = "33333333-3333-3333-3333-333333333333"
	testHotelID  = "44444444-4444-4444-4444-444444444444"
	testRoomID   = "55555555-5555-5555-5555-555555555555"

	seedFlightSeats = 100
	seedRoomCount   = 10

	pollInterval = 500 * time.Millisecond
	pollTimeout  = 30 * time.Second
)

type SagaSuite struct {
	suite.Suite

	cfg *Config

	bookingDB  *sqlx.DB
	flightsDB  *sqlx.DB
	hotelsDB   *sqlx.DB

	createdBookingIDs []string
}

func TestSaga(t *testing.T) {
	suite.Run(t, new(SagaSuite))
}

func (s *SagaSuite) SetupSuite() {
	cfg, err := loadConfig("config.yaml")
	s.Require().NoError(err, "load config")
	s.cfg = cfg

	s.bookingDB, err = sqlx.Connect("pgx", cfg.BookingDB.PostgresDSN())
	s.Require().NoError(err, "connect booking_db")

	s.flightsDB, err = sqlx.Connect("pgx", cfg.FlightsDB.PostgresDSN())
	s.Require().NoError(err, "connect flights_db")

	s.hotelsDB, err = sqlx.Connect("mysql", cfg.HotelsDB.MySQLDSN())
	s.Require().NoError(err, "connect hotels_db")
}

func (s *SagaSuite) TearDownSuite() {
	if s.bookingDB != nil {
		s.bookingDB.Close()
	}
	if s.flightsDB != nil {
		s.flightsDB.Close()
	}
	if s.hotelsDB != nil {
		s.hotelsDB.Close()
	}
}

func (s *SagaSuite) TearDownTest() {
	for _, id := range s.createdBookingIDs {
		s.bookingDB.Exec("DELETE FROM bookings WHERE id = $1", id)
	}
	s.createdBookingIDs = nil

	s.flightsDB.Exec(
		"DELETE FROM flight.flight_bookings WHERE user_id = $1", testUserID,
	)
	s.hotelsDB.Exec(
		"DELETE FROM hotel_bookings WHERE user_id = ?", testUserID,
	)

	s.flightsDB.Exec(
		"UPDATE flight.flights SET available_seats = $1, updated_at = NOW() WHERE id = $2",
		seedFlightSeats, testFlightID,
	)
	s.hotelsDB.Exec(
		"UPDATE hotel_rooms SET rooms_available = ? WHERE id = ?",
		seedRoomCount, testRoomID,
	)
}

type createBookingRequest struct {
	UserID      string `json:"user_id"`
	FlightID    string `json:"flight_id"`
	HotelID     string `json:"hotel_id"`
	RoomID      string `json:"room_id"`
	CheckIn     string `json:"check_in"`
	CheckOut    string `json:"check_out"`
	AmountCents int    `json:"amount_cents"`
	Currency    string `json:"currency"`
}

type createBookingResponse struct {
	BookingID string `json:"booking_id"`
}

func (s *SagaSuite) createBooking(req createBookingRequest) string {
	body, err := json.Marshal(req)
	s.Require().NoError(err)

	url := s.cfg.Services.Booking.Address + "/api/v1/bookings"
	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	s.Require().NoError(err)
	defer resp.Body.Close()

	s.Require().Equal(http.StatusCreated, resp.StatusCode, "POST /api/v1/bookings must return 201")

	var result createBookingResponse
	s.Require().NoError(json.NewDecoder(resp.Body).Decode(&result))
	s.Require().NotEmpty(result.BookingID)

	s.createdBookingIDs = append(s.createdBookingIDs, result.BookingID)
	return result.BookingID
}

func (s *SagaSuite) waitForBookingStatus(bookingID, expectedStatus string) {
	s.T().Helper()

	require.Eventually(s.T(), func() bool {
		var status string
		err := s.bookingDB.QueryRow(
			"SELECT payment_status FROM bookings WHERE id = $1", bookingID,
		).Scan(&status)
		if err != nil {
			return false
		}
		return status == expectedStatus
	}, pollTimeout, pollInterval, fmt.Sprintf("booking %s did not reach status %q", bookingID, expectedStatus))
}

func (s *SagaSuite) waitForBookingCancelled(bookingID string) {
	s.T().Helper()

	require.Eventually(s.T(), func() bool {
		var cancelled bool
		err := s.bookingDB.QueryRow(
			"SELECT is_cancelled FROM bookings WHERE id = $1", bookingID,
		).Scan(&cancelled)
		if err != nil {
			return false
		}
		return cancelled
	}, pollTimeout, pollInterval, fmt.Sprintf("booking %s was not cancelled", bookingID))
}

func (s *SagaSuite) getFlightAvailableSeats(flightID string) int {
	s.T().Helper()
	var seats int
	err := s.flightsDB.QueryRow(
		"SELECT available_seats FROM flight.flights WHERE id = $1", flightID,
	).Scan(&seats)
	s.Require().NoError(err)
	return seats
}

func (s *SagaSuite) getFlightBookingStatus(userID, flightID string) string {
	s.T().Helper()
	var status string
	err := s.flightsDB.QueryRow(
		"SELECT status FROM flight.flight_bookings WHERE user_id = $1 AND flight_id = $2 ORDER BY created_at DESC LIMIT 1",
		userID, flightID,
	).Scan(&status)
	if err == sql.ErrNoRows {
		return ""
	}
	s.Require().NoError(err)
	return status
}

func (s *SagaSuite) getRoomAvailable(roomID string) int {
	s.T().Helper()
	var count int
	err := s.hotelsDB.QueryRow(
		"SELECT rooms_available FROM hotel_rooms WHERE id = ?", roomID,
	).Scan(&count)
	s.Require().NoError(err)
	return count
}

func (s *SagaSuite) getHotelBookingStatus(userID, hotelID string) string {
	s.T().Helper()
	var status string
	err := s.hotelsDB.QueryRow(
		"SELECT status FROM hotel_bookings WHERE user_id = ? AND hotel_id = ? ORDER BY created_at DESC LIMIT 1",
		userID, hotelID,
	).Scan(&status)
	if err == sql.ErrNoRows {
		return ""
	}
	s.Require().NoError(err)
	return status
}

func (s *SagaSuite) getBookingCancelReason(bookingID string) string {
	s.T().Helper()
	var reason sql.NullString
	err := s.bookingDB.QueryRow(
		"SELECT cancel_reason FROM bookings WHERE id = $1", bookingID,
	).Scan(&reason)
	s.Require().NoError(err)
	if reason.Valid {
		return reason.String
	}
	return ""
}
