ALTER TABLE metric_histograms_1h ADD COLUMN IF NOT EXISTS weight_squares SimpleAggregateFunction(sum, Float64) DEFAULT cnt * 1000
