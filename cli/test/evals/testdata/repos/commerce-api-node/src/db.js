import pg from 'pg';
import Redis from 'ioredis';

export const pool = new pg.Pool({ connectionString: process.env.DATABASE_URL, max: 20 });
export const redis = new Redis(process.env.REDIS_URL);
