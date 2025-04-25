-- Create the database (optional, depends on your setup)
CREATE DATABASE dhcpdb;

-- Switch to the database
\c dhcpdb;

-- SQL Script to Initialize the Database Schema

-- Table for storing IP pools
CREATE TABLE ip_pools (
    id SERIAL PRIMARY KEY,
    cidr TEXT NOT NULL UNIQUE, -- CIDR range for the pool
    gateway TEXT NOT NULL UNIQUE, -- Gateway range for the pool
    description TEXT
);

-- Table for storing leases
CREATE TABLE leases (
    ip_address TEXT PRIMARY KEY,
    mac_address TEXT NOT NULL,
    hostname TEXT,
    lease_start TIMESTAMP NOT NULL,
    lease_end TIMESTAMP NOT NULL,
    binding_state TEXT NOT NULL, -- e.g., "active", "expired", "free"
    last_transaction TIMESTAMP,
    next_binding_state TEXT,
    bootfile_url TEXT,
    tftp_server TEXT,
    ip_pool_id INT REFERENCES ip_pools(id) ON DELETE SET NULL
);

-- Sample data
INSERT INTO ip_pools (cidr, gateway, description) VALUES
('192.168.1.0/24', '192.168.1.1', 'Default Pool'),
('10.0.0.0/16', '10.0.0.1', 'Corporate Pool'),
('10.70.1.0/24', '10.70.1.1', 'BMW Pool'),
('10.80.1.0/24', '10.80.1.1', 'NUC Pool');

-- INSERT INTO leases (
--     ip_address, mac_address, hostname, lease_start, lease_end,
--     binding_state, last_transaction, next_binding_state, bootfile_url,
--     tftp_server, ip_pool_id
-- ) VALUES
-- ('10.70.1.2', '00:11:22:33:44:55', 'test-host', NOW(), NOW() + INTERVAL '1 hour',
--  'inactive', NOW(), 'released', '/path/to/bootfile', '10.70.1.1', 1),
-- ('10.70.1.3', '00:11:22:33:44:55', 'test-host', NOW(), NOW() + INTERVAL '1 hour',
--  'inactive', NOW(), 'released', '/path/to/bootfile', '10.70.1.1', 1),
-- ('10.70.1.4', '00:11:22:33:44:55', 'test-host', NOW(), NOW() + INTERVAL '1 hour',
--  'inactive', NOW(), 'released', '/path/to/bootfile', '10.70.1.1', 1),
-- ('10.70.1.5', '00:11:22:33:44:55', 'test-host', NOW(), NOW() + INTERVAL '1 hour',
--  'inactive', NOW(), 'released', '/path/to/bootfile', '10.70.1.1', 1),
-- ('10.80.1.100', '94:c6:91:1e:95:4b', 'intel-nuc-00', NOW(), NOW() + INTERVAL '1 hour',
-- 'inactive', NOW(), 'released', 'pxelinux.0', '10.80.1.1', 3),
-- ('10.70.1.20', '52:54:00:b4:a9:ca', 'kvm-00', NOW(), NOW() + INTERVAL '1 hour',
-- 'inactive', NOW(), 'released', 'pxelinux.0', '10.70.1.1', 2),
-- ('192.168.1.1', '00:11:22:33:44:55', 'test-host', NOW(), NOW() + INTERVAL '1 hour',
--  'inactive', NOW(), 'released', '/path/to/bootfile', '192.168.1.1', 1);
 

