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

// Provider fetches airport and distance data from an external API.
type Provider interface {
	GetAirportByIATA(ctx context.Context, iata string) (*domain.Airport, error)
	GetDistanceBetweenAirports(ctx context.Context, fromIATA, toIATA string) (*domain.GreatCircleDistance, error)
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

// GetDistanceBetweenAirports returns route distance, consulting the cache before the API.
func (c *CachedLookup) GetDistanceBetweenAirports(ctx context.Context, fromIATA, toIATA string) (*domain.GreatCircleDistance, error) {
	d, err := c.store.FindDistance(ctx, fromIATA, toIATA)
	if err == nil {
		return d, nil
	}

	time.Sleep(apiThrottleDelay)

	d, err = c.provider.GetDistanceBetweenAirports(ctx, fromIATA, toIATA)
	if err != nil {
		return nil, fmt.Errorf("distance lookup %s-%s: %w", fromIATA, toIATA, err)
	}

	saveErr := c.store.SaveDistance(ctx, fromIATA, toIATA, d)
	if saveErr != nil {
		c.log.Warn("failed to cache distance",
			zap.String("route", fromIATA+"-"+toIATA),
			zap.Error(saveErr),
		)
	}

	return d, nil
}

// BackfillFromFlight populates the cache from data already present in a flight.
func (c *CachedLookup) BackfillFromFlight(ctx context.Context, f *domain.Flight) {
	c.backfillAirport(ctx, &f.Departure.Airport)
	c.backfillAirport(ctx, &f.Arrival.Airport)
	c.backfillDistance(ctx, f)
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

func (c *CachedLookup) backfillDistance(ctx context.Context, f *domain.Flight) {
	dep := f.Departure.Airport.IATA
	arr := f.Arrival.Airport.IATA

	if f.GreatCircleDistance.Km <= 0 || dep == "" || arr == "" {
		return
	}

	err := c.store.SaveDistance(ctx, dep, arr, &f.GreatCircleDistance)
	if err != nil {
		c.log.Debug("backfill distance failed", zap.String("route", dep+"-"+arr), zap.Error(err))
	}
}
