CREATE TABLE LMS.LMS_AlertTriggers
(
    `Trigger_ID` String,
    `AlertRule_ID` String,
    `Rule_Name` String,
    `Trigger_Time` DateTime DEFAULT now(),
    `Match_Count` UInt32 DEFAULT 0,
    `Channel` String DEFAULT '',
    `Address` String DEFAULT '',
    `Message` String DEFAULT ''
)
ENGINE = MergeTree
ORDER BY (Trigger_Time)
SETTINGS index_granularity = 8192
