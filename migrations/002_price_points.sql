-- PriceAlert v1 price points schema
-- Adds persisted grouped-market history points for snapshot-driven history.

CREATE TABLE price_points (
    id CHAR(26) NOT NULL,
    tracked_keyword_id CHAR(26) NOT NULL,
    scan_job_id CHAR(26) NOT NULL,
    min_price BIGINT NULL,
    avg_price BIGINT NULL,
    max_price BIGINT NULL,
    recorded_at DATETIME NOT NULL,
    PRIMARY KEY (id),
    KEY idx_price_points_tracked_keyword_id (tracked_keyword_id),
    KEY idx_price_points_tracked_keyword_recorded_at (tracked_keyword_id, recorded_at),
    CONSTRAINT fk_price_points_tracked_keyword
        FOREIGN KEY (tracked_keyword_id) REFERENCES tracked_keywords (id),
    CONSTRAINT fk_price_points_scan_job
        FOREIGN KEY (scan_job_id) REFERENCES scan_jobs (id)
        ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
