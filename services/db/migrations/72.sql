-- Migration: 071_add_news_tweets_table
-- Description: Add news_tweets table for storing Twitter webhook data with indexes for fast querying

BEGIN;

-- Create news_tweets table
CREATE TABLE IF NOT EXISTS news_tweets (
    id SERIAL PRIMARY KEY,
    tweet_text TEXT NOT NULL,
    created_at TEXT NOT NULL,
    url TEXT NOT NULL,
    username TEXT NOT NULL,
    inserted_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Add indexes for fast querying
CREATE INDEX IF NOT EXISTS idx_news_tweets_username ON news_tweets(username);
CREATE INDEX IF NOT EXISTS idx_news_tweets_created_at ON news_tweets(created_at);
CREATE INDEX IF NOT EXISTS idx_news_tweets_tweet_text ON news_tweets USING gin(to_tsvector('english', tweet_text));
CREATE INDEX IF NOT EXISTS idx_news_tweets_inserted_at ON news_tweets(inserted_at);

COMMIT; 