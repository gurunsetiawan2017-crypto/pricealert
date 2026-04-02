-- PriceAlert v1 initial schema
-- Phase 1 foundation: minimum practical persistence for tracked keywords,
-- scan jobs, raw/grouped listings, market snapshots, and alert events.

CREATE TABLE tracked_keywords (
    id CHAR(26) NOT NULL,
    keyword VARCHAR(255) NOT NULL,
    basic_filter VARCHAR(100) NULL,
    threshold_price BIGINT NULL,
    interval_minutes INT NOT NULL,
    telegram_enabled BOOLEAN NOT NULL,
    status VARCHAR(20) NOT NULL,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    PRIMARY KEY (id),
    KEY idx_tracked_keywords_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE scan_jobs (
    id CHAR(26) NOT NULL,
    tracked_keyword_id CHAR(26) NOT NULL,
    started_at DATETIME NOT NULL,
    finished_at DATETIME NULL,
    status VARCHAR(20) NOT NULL,
    error_message TEXT NULL,
    raw_count INT NULL,
    grouped_count INT NULL,
    PRIMARY KEY (id),
    KEY idx_scan_jobs_tracked_keyword_id (tracked_keyword_id),
    KEY idx_scan_jobs_tracked_keyword_started_at (tracked_keyword_id, started_at),
    CONSTRAINT fk_scan_jobs_tracked_keyword
        FOREIGN KEY (tracked_keyword_id) REFERENCES tracked_keywords (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE raw_listings (
    id CHAR(26) NOT NULL,
    scan_job_id CHAR(26) NOT NULL,
    source VARCHAR(50) NOT NULL,
    title TEXT NOT NULL,
    normalized_title TEXT NOT NULL,
    seller_name VARCHAR(255) NOT NULL,
    price BIGINT NOT NULL,
    original_price BIGINT NULL,
    is_promo BOOLEAN NOT NULL,
    url TEXT NOT NULL,
    scraped_at DATETIME NOT NULL,
    PRIMARY KEY (id),
    KEY idx_raw_listings_scan_job_id (scan_job_id),
    CONSTRAINT fk_raw_listings_scan_job
        FOREIGN KEY (scan_job_id) REFERENCES scan_jobs (id)
        ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE grouped_listings (
    id CHAR(26) NOT NULL,
    scan_job_id CHAR(26) NOT NULL,
    group_key VARCHAR(255) NOT NULL,
    representative_title TEXT NOT NULL,
    representative_seller VARCHAR(255) NOT NULL,
    best_price BIGINT NOT NULL,
    original_price BIGINT NULL,
    is_promo BOOLEAN NOT NULL,
    listing_count INT NOT NULL,
    sample_url TEXT NOT NULL,
    PRIMARY KEY (id),
    KEY idx_grouped_listings_scan_job_id (scan_job_id),
    KEY idx_grouped_listings_scan_job_best_price (scan_job_id, best_price),
    CONSTRAINT fk_grouped_listings_scan_job
        FOREIGN KEY (scan_job_id) REFERENCES scan_jobs (id)
        ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE market_snapshots (
    id CHAR(26) NOT NULL,
    tracked_keyword_id CHAR(26) NOT NULL,
    scan_job_id CHAR(26) NOT NULL,
    min_price BIGINT NULL,
    avg_price BIGINT NULL,
    max_price BIGINT NULL,
    raw_count INT NOT NULL,
    grouped_count INT NOT NULL,
    `signal` VARCHAR(20) NOT NULL,
    snapshot_at DATETIME NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_market_snapshots_scan_job_id (scan_job_id),
    KEY idx_market_snapshots_tracked_keyword_id (tracked_keyword_id),
    KEY idx_market_snapshots_tracked_keyword_snapshot_at (tracked_keyword_id, snapshot_at),
    CONSTRAINT fk_market_snapshots_tracked_keyword
        FOREIGN KEY (tracked_keyword_id) REFERENCES tracked_keywords (id),
    CONSTRAINT fk_market_snapshots_scan_job
        FOREIGN KEY (scan_job_id) REFERENCES scan_jobs (id)
        ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE alert_events (
    id CHAR(26) NOT NULL,
    tracked_keyword_id CHAR(26) NOT NULL,
    scan_job_id CHAR(26) NULL,
    level VARCHAR(20) NOT NULL,
    event_type VARCHAR(50) NOT NULL,
    message TEXT NOT NULL,
    payload_json LONGTEXT NULL,
    sent_to_telegram BOOLEAN NOT NULL,
    created_at DATETIME NOT NULL,
    PRIMARY KEY (id),
    KEY idx_alert_events_tracked_keyword_id (tracked_keyword_id),
    KEY idx_alert_events_scan_job_id (scan_job_id),
    KEY idx_alert_events_tracked_keyword_created_at (tracked_keyword_id, created_at),
    CONSTRAINT fk_alert_events_tracked_keyword
        FOREIGN KEY (tracked_keyword_id) REFERENCES tracked_keywords (id),
    CONSTRAINT fk_alert_events_scan_job
        FOREIGN KEY (scan_job_id) REFERENCES scan_jobs (id)
        ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
