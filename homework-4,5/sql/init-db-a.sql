-- Database A initialization script
-- Contains: users, payments, outbox (for outgoing events)

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Пользователи системы
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(100) NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Платежи пользователей
CREATE TABLE payments (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id),
    amount DECIMAL(10,2) NOT NULL,
    currency VARCHAR(3) DEFAULT 'USD',
    description TEXT,
    status VARCHAR(20) DEFAULT 'pending',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Outbox для исходящих событий репликации
CREATE TABLE outbox (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    aggregate_id UUID NOT NULL,          -- ID основной записи (user_id или payment_id)
    aggregate_type VARCHAR(50) NOT NULL, -- 'user' или 'payment'
    event_type VARCHAR(50) NOT NULL,     -- 'created', 'updated', 'deleted'
    event_data JSONB NOT NULL,           -- JSON с данными события
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    processed BOOLEAN DEFAULT FALSE,
    processed_at TIMESTAMP WITH TIME ZONE NULL
);

-- Индексы для производительности
CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_payments_user_id ON payments(user_id);
CREATE INDEX idx_payments_status ON payments(status);
CREATE INDEX idx_payments_created_at ON payments(created_at);

CREATE INDEX idx_outbox_processed ON outbox(processed);
CREATE INDEX idx_outbox_created_at ON outbox(created_at);
CREATE INDEX idx_outbox_aggregate ON outbox(aggregate_type, aggregate_id);

-- Триггеры для автоматического создания событий в outbox
-- При создании пользователя
CREATE OR REPLACE FUNCTION trigger_user_outbox()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        INSERT INTO outbox (aggregate_id, aggregate_type, event_type, event_data)
        VALUES (
            NEW.id,
            'user',
            'created',
            json_build_object(
                'id', NEW.id,
                'name', NEW.name,
                'email', NEW.email,
                'created_at', NEW.created_at
            )
        );
    ELSIF TG_OP = 'UPDATE' THEN
        INSERT INTO outbox (aggregate_id, aggregate_type, event_type, event_data)
        VALUES (
            NEW.id,
            'user',
            'updated',
            json_build_object(
                'id', NEW.id,
                'name', NEW.name,
                'email', NEW.email,
                'updated_at', NEW.updated_at
            )
        );
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- При создании платежа
CREATE OR REPLACE FUNCTION trigger_payment_outbox()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        INSERT INTO outbox (aggregate_id, aggregate_type, event_type, event_data)
        VALUES (
            NEW.id,
            'payment',
            'created',
            json_build_object(
                'id', NEW.id,
                'user_id', NEW.user_id,
                'amount', NEW.amount,
                'currency', NEW.currency,
                'description', NEW.description,
                'status', NEW.status,
                'created_at', NEW.created_at
            )
        );
    ELSIF TG_OP = 'UPDATE' THEN
        INSERT INTO outbox (aggregate_id, aggregate_type, event_type, event_data)
        VALUES (
            NEW.id,
            'payment',
            'updated',
            json_build_object(
                'id', NEW.id,
                'user_id', NEW.user_id,
                'amount', NEW.amount,
                'currency', NEW.currency,
                'description', NEW.description,
                'status', NEW.status,
                'updated_at', NEW.updated_at
            )
        );
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Создание триггеров
CREATE TRIGGER users_outbox_trigger
    AFTER INSERT OR UPDATE ON users
    FOR EACH ROW
    EXECUTE FUNCTION trigger_user_outbox();

CREATE TRIGGER payments_outbox_trigger
    AFTER INSERT OR UPDATE ON payments
    FOR EACH ROW
    EXECUTE FUNCTION trigger_payment_outbox();

-- Функция для отметки событий как обработанных
CREATE OR REPLACE FUNCTION mark_outbox_processed(event_ids UUID[])
RETURNS INTEGER AS $$
DECLARE
    updated_count INTEGER;
BEGIN
    UPDATE outbox 
    SET processed = TRUE, processed_at = NOW()
    WHERE id = ANY(event_ids) AND processed = FALSE;
    
    GET DIAGNOSTICS updated_count = ROW_COUNT;
    RETURN updated_count;
END;
$$ LANGUAGE plpgsql; 