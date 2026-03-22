package integration

func (s *SagaSuite) TestHappyPath() {
	seatsBefore := s.getFlightAvailableSeats(testFlightID)
	roomsBefore := s.getRoomAvailable(testRoomID)

	bookingID := s.createBooking(createBookingRequest{
		UserID:      testUserID,
		FlightID:    testFlightID,
		HotelID:     testHotelID,
		RoomID:      testRoomID,
		CheckIn:     "2026-06-01",
		CheckOut:    "2026-06-05",
		AmountCents: 500000,
		Currency:    "RUB",
	})

	s.waitForBookingStatus(bookingID, "charged")

	seatsAfter := s.getFlightAvailableSeats(testFlightID)
	s.Equal(seatsBefore-1, seatsAfter, "available_seats must decrease by 1")

	flightStatus := s.getFlightBookingStatus(testUserID, testFlightID)
	s.Equal("reserved", flightStatus, "flight booking must be reserved")

	roomsAfter := s.getRoomAvailable(testRoomID)
	s.Equal(roomsBefore-1, roomsAfter, "rooms_available must decrease by 1")

	hotelStatus := s.getHotelBookingStatus(testUserID, testHotelID)
	s.Equal("reserved", hotelStatus, "hotel booking must be reserved")
}

func (s *SagaSuite) TestCompensation_HotelUnavailable() {
	_, err := s.hotelsDB.Exec(
		"UPDATE hotel_rooms SET rooms_available = 0 WHERE id = ?", testRoomID,
	)
	s.Require().NoError(err, "block room availability")

	seatsBefore := s.getFlightAvailableSeats(testFlightID)

	bookingID := s.createBooking(createBookingRequest{
		UserID:      testUserID,
		FlightID:    testFlightID,
		HotelID:     testHotelID,
		RoomID:      testRoomID,
		CheckIn:     "2026-06-01",
		CheckOut:    "2026-06-05",
		AmountCents: 500000,
		Currency:    "RUB",
	})

	s.waitForBookingCancelled(bookingID)

	cancelReason := s.getBookingCancelReason(bookingID)
	s.Equal("saga_failed", cancelReason, "cancel_reason must be saga_failed")

	flightStatus := s.getFlightBookingStatus(testUserID, testFlightID)
	s.Equal("cancelled", flightStatus, "flight booking must be cancelled after compensation")

	seatsAfter := s.getFlightAvailableSeats(testFlightID)
	s.Equal(seatsBefore, seatsAfter, "available_seats must be restored after compensation")
}
