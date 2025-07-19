-- Database B initialization script  
-- Contains: users, payments, inbox (for incoming replicated events)

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Пользователи системы (реплицированные из Database A)
CREATE TABLE users (
    id UUID PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL,
    -- Метаданные репликации
    replicated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Платежи пользователей (реплицированные из Database A)
CREATE TABLE payments (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id),
    amount DECIMAL(10,2) NOT NULL,
    currency VARCHAR(3) DEFAULT 'USD',
    description TEXT,
    status VARCHAR(20) DEFAULT 'pending',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL,
    -- Метаданные репликации
    replicated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Inbox для входящих событий репликации
CREATE TABLE inbox (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    event_id UUID UNIQUE NOT NULL,       -- ID события из outbox Database A
    aggregate_id UUID NOT NULL,          -- ID основной записи (user_id или payment_id)
    aggregate_type VARCHAR(50) NOT NULL, -- 'user' или 'payment'
    event_type VARCHAR(50) NOT NULL,     -- 'created', 'updated', 'deleted'
    event_data JSONB NOT NULL,           -- JSON с данными события
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    processed BOOLEAN DEFAULT FALSE,
    processed_at TIMESTAMP WITH TIME ZONE NULL
);

-- Таблица для отслеживания обработанных событий (дедупликация)
CREATE TABLE processed_events (
    event_id UUID PRIMARY KEY,
    aggregate_id UUID NOT NULL,
    aggregate_type VARCHAR(50) NOT NULL,
    event_type VARCHAR(50) NOT NULL,
    processed_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Индексы для производительности
CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_replicated_at ON users(replicated_at);

CREATE INDEX idx_payments_user_id ON payments(user_id);
CREATE INDEX idx_payments_status ON payments(status);
CREATE INDEX idx_payments_created_at ON payments(created_at);
CREATE INDEX idx_payments_replicated_at ON payments(replicated_at);

CREATE INDEX idx_inbox_processed ON inbox(processed);
CREATE INDEX idx_inbox_created_at ON inbox(created_at);
CREATE INDEX idx_inbox_event_id ON inbox(event_id);
CREATE INDEX idx_inbox_aggregate ON inbox(aggregate_type, aggregate_id);

CREATE INDEX idx_processed_events_aggregate ON processed_events(aggregate_type, aggregate_id);
CREATE INDEX idx_processed_events_processed_at ON processed_events(processed_at);

-- Функция для проверки, было ли событие уже обработано
CREATE OR REPLACE FUNCTION is_event_processed(event_uuid UUID)
RETURNS BOOLEAN AS $$
BEGIN
    RETURN EXISTS (
        SELECT 1 FROM processed_events 
        WHERE event_id = event_uuid
    );
END;
$$ LANGUAGE plpgsql;

-- Функция для обработки события пользователя
CREATE OR REPLACE FUNCTION process_user_event(
    p_event_id UUID,
    p_event_type VARCHAR(50),
    p_event_data JSONB
)
RETURNS BOOLEAN AS $$
DECLARE
    user_exists BOOLEAN;
BEGIN
    -- Проверяем, не обработано ли уже это событие
    IF is_event_processed(p_event_id) THEN
        RETURN FALSE; -- Событие уже обработано
    END IF;

    -- Извлекаем данные пользователя из JSON
    IF p_event_type = 'created' THEN
        INSERT INTO users (id, name, email, created_at, updated_at)
        VALUES (
            (p_event_data->>'id')::UUID,
            p_event_data->>'name',
            p_event_data->>'email',
            (p_event_data->>'created_at')::TIMESTAMP WITH TIME ZONE,
            (p_event_data->>'created_at')::TIMESTAMP WITH TIME ZONE
        )
        ON CONFLICT (id) DO NOTHING;
        
    ELSIF p_event_type = 'updated' THEN
        UPDATE users SET
            name = p_event_data->>'name',
            email = p_event_data->>'email',
            updated_at = (p_event_data->>'updated_at')::TIMESTAMP WITH TIME ZONE
        WHERE id = (p_event_data->>'id')::UUID;
    END IF;

    -- Отмечаем событие как обработанное
    INSERT INTO processed_events (event_id, aggregate_id, aggregate_type, event_type)
    VALUES (
        p_event_id,
        (p_event_data->>'id')::UUID,
        'user',
        p_event_type
    );

    RETURN TRUE;
END;
$$ LANGUAGE plpgsql;

-- Функция для обработки события платежа
CREATE OR REPLACE FUNCTION process_payment_event(
    p_event_id UUID,
    p_event_type VARCHAR(50), 
    p_event_data JSONB
)
RETURNS BOOLEAN AS $$
BEGIN
    -- Проверяем, не обработано ли уже это событие
    IF is_event_processed(p_event_id) THEN
        RETURN FALSE; -- Событие уже обработано
    END IF;

    -- Извлекаем данные платежа из JSON
    IF p_event_type = 'created' THEN
        INSERT INTO payments (id, user_id, amount, currency, description, status, created_at, updated_at)
        VALUES (
            (p_event_data->>'id')::UUID,
            (p_event_data->>'user_id')::UUID,
            (p_event_data->>'amount')::DECIMAL(10,2),
            p_event_data->>'currency',
            p_event_data->>'description',
            p_event_data->>'status',
            (p_event_data->>'created_at')::TIMESTAMP WITH TIME ZONE,
            (p_event_data->>'created_at')::TIMESTAMP WITH TIME ZONE
        )
        ON CONFLICT (id) DO NOTHING;
        
    ELSIF p_event_type = 'updated' THEN
        UPDATE payments SET
            user_id = (p_event_data->>'user_id')::UUID,
            amount = (p_event_data->>'amount')::DECIMAL(10,2),
            currency = p_event_data->>'currency',
            description = p_event_data->>'description',
            status = p_event_data->>'status',
            updated_at = (p_event_data->>'updated_at')::TIMESTAMP WITH TIME ZONE
        WHERE id = (p_event_data->>'id')::UUID;
    END IF;

    -- Отмечаем событие как обработанное
    INSERT INTO processed_events (event_id, aggregate_id, aggregate_type, event_type)
    VALUES (
        p_event_id,
        (p_event_data->>'id')::UUID,
        'payment',
        p_event_type
    );

    RETURN TRUE;
END;
$$ LANGUAGE plpgsql;

-- Функция для отметки событий inbox как обработанных
CREATE OR REPLACE FUNCTION mark_inbox_processed(event_ids UUID[])
RETURNS INTEGER AS $$
DECLARE
    updated_count INTEGER;
BEGIN
    UPDATE inbox 
    SET processed = TRUE, processed_at = NOW()
    WHERE id = ANY(event_ids) AND processed = FALSE;
    
    GET DIAGNOSTICS updated_count = ROW_COUNT;
    RETURN updated_count;
END;
$$ LANGUAGE plpgsql; 