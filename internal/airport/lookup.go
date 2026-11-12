package airport

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/thulasirajkomminar/flightlog/internal/domain"
	"github.com/thulasirajkomminar/flightlog/internal/logger"
)

const apiThrottleDelay = 1 * time.Second

// Provider fetches airport data from an external API.
type Provider interface {
	GetAirportByIATA(ctx context.Context, iata string) (*domain.Airport, error)
}

// CachedLookup decorates a Provider with DB-backed caching and rate limiting.
type CachedLookup struct {
	store    *Store
	provider Provider
	log      *logger.ComponentLogger
}

// NewCachedLookup returns a lookup that caches results in the given store.
func NewCachedLookup(store *Store, provider Provider) *CachedLookup {
	return &CachedLookup{
		store:    store,
		provider: provider,
		log:      logger.NewComponentLogger("airport_cache"),
	}
}

// GetAirportByIATA returns airport data, consulting the cache before the API.
func (c *CachedLookup) GetAirportByIATA(ctx context.Context, iata string) (*domain.Airport, error) {
	a, err := c.store.FindAirport(ctx, iata)
	if err == nil {
		return a, nil
	}

	time.Sleep(apiThrottleDelay)

	a, err = c.provider.GetAirportByIATA(ctx, iata)
	if err != nil {
		return nil, fmt.Errorf("airport lookup %s: %w", iata, err)
	}

	saveErr := c.store.SaveAirport(ctx, a)
	if saveErr != nil {
		c.log.Warn("failed to cache airport",
			zap.String("iata", iata),
			zap.Error(saveErr),
		)
	}

	return a, nil
}

// BackfillFromFlight populates the cache from data already present in a flight.
func (c *CachedLookup) BackfillFromFlight(ctx context.Context, f *domain.Flight) {
	c.backfillAirport(ctx, &f.Departure.Airport)
	c.backfillAirport(ctx, &f.Arrival.Airport)
}

func (c *CachedLookup) backfillAirport(ctx context.Context, a *domain.Airport) {
	if a.IATA == "" || a.Name == "" {
		return
	}

	err := c.store.SaveAirport(ctx, a)
	if err != nil {
		c.log.Debug("backfill airport failed", zap.String("iata", a.IATA), zap.Error(err))
	}
}
