package svc

import "errors"

// ErrNotImplemented is returned by service stubs that have not yet been wired
// to real database queries. Handlers map this to HTTP 501 so the API surface
// remains discoverable while implementation lands incrementally.
var ErrNotImplemented = errors.New("not implemented")

// ErrNotFound is returned by services when the requested entity does not exist
// (planet, fleet, message, etc.). Handlers translate this to HTTP 404.
var ErrNotFound = errors.New("not found")

// ErrForbidden is returned when an action is rejected because the caller does
// not own the target resource (planet, fleet, message). Handlers map to 403.
var ErrForbidden = errors.New("forbidden")

// ErrInsufficientResources is returned by build/research/shipyard services
// when the planet lacks the resources to queue the requested item. Handlers
// surface this as 409 Conflict with a descriptive message.
var ErrInsufficientResources = errors.New("insufficient resources")

// ErrPrerequisiteNotMet is returned when a building, ship, or research is
// requested without meeting the tech-tree prerequisites.
var ErrPrerequisiteNotMet = errors.New("prerequisite not met")

// ErrQueueBusy is returned when the construction or research slot is already
// full and the request must wait.
var ErrQueueBusy = errors.New("queue busy")
