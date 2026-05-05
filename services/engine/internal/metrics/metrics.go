package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	HandsStarted = promauto.NewCounter(prometheus.CounterOpts{
		Name: "holdem_hands_started_total",
		Help: "Total number of hands started",
	})

	HandsEnded = promauto.NewCounter(prometheus.CounterOpts{
		Name: "holdem_hands_ended_total",
		Help: "Total number of hands ended",
	})

	ActionsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "holdem_actions_total",
		Help: "Total number of player actions by type",
	}, []string{"type"})

	PotSize = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "holdem_pot_size_chips",
		Help:    "Distribution of pot sizes in chips",
		Buckets: []float64{50, 100, 250, 500, 1000, 2500, 5000, 10000},
	})

	RakeCollected = promauto.NewCounter(prometheus.CounterOpts{
		Name: "holdem_rake_collected_total",
		Help: "Total rake collected in chips",
	})

	HandDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "holdem_hand_duration_seconds",
		Help:    "Duration of a hand in seconds",
		Buckets: []float64{5, 10, 30, 60, 120, 300},
	})

	ActiveGames = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "holdem_active_games",
		Help: "Current number of active games",
	})

	ShowdownsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "holdem_showdowns_total",
		Help: "Total number of hands that reached showdown",
	})

	PlayersPerHand = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "holdem_players_per_hand",
		Help:    "Number of players per hand",
		Buckets: []float64{2, 3, 4, 5, 6, 7, 8, 9},
	})

	KafkaPublishTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "holdem_kafka_publish_total",
		Help: "Total number of kafka messages published by topic",
	}, []string{"topic", "status"})

	DBQueryDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "holdem_db_query_duration_seconds",
		Help:    "Duration of database queries",
		Buckets: prometheus.DefBuckets,
	}, []string{"operation"})

	PgxPoolAcquired = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "holdem_pgx_pool_acquired_conns",
		Help: "Number of currently acquired pgx pool connections",
	})

	PgxPoolIdle = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "holdem_pgx_pool_idle_conns",
		Help: "Number of idle pgx pool connections",
	})
)