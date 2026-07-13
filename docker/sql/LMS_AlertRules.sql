CREATE TABLE LMS.LMS_AlertRules
(
    `AlertRule_ID` String,
    `Name` String DEFAULT '',
    `Desc` String DEFAULT '',
    `Alert_Sql` String DEFAULT '',
    `Interval` String DEFAULT '',
    `Channel` String DEFAULT '',
    `Address` String DEFAULT '',
    `Created_Time` DateTime DEFAULT now(),
    `Updated_Time` DateTime DEFAULT now(),
    `Level` String DEFAULT '',
    `Status` String DEFAULT '1'
)
ENGINE = MergeTree
ORDER BY (AlertRule_ID)
SETTINGS index_granularity = 8192
