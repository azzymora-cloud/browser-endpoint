-- Seed browser connections for this Windows desktop.
-- Credentials are not stored: Guacamole prompts at connect time.
-- Use RDP on Windows Pro/Enterprise; use VNC on Windows Home.

INSERT INTO guacamole_connection (connection_name, protocol)
VALUES
    ('Cybertron (RDP)', 'rdp'),
    ('Cybertron (VNC)', 'vnc');

-- RDP: Docker Desktop reaches the Windows host via host.docker.internal.
INSERT INTO guacamole_connection_parameter (connection_id, parameter_name, parameter_value)
SELECT connection_id, parameter_name, parameter_value
FROM guacamole_connection
CROSS JOIN (
    VALUES
        ('hostname', 'host.docker.internal'),
        ('port', '3389'),
        ('ignore-cert', 'true'),
        ('security', 'nla'),
        ('resize-method', 'display-update'),
        ('enable-font-smoothing', 'true'),
        ('disable-audio', 'true')
) AS params(parameter_name, parameter_value)
WHERE connection_name = 'Cybertron (RDP)';

INSERT INTO guacamole_connection_parameter (connection_id, parameter_name, parameter_value)
SELECT connection_id, parameter_name, parameter_value
FROM guacamole_connection
CROSS JOIN (
    VALUES
        ('hostname', 'host.docker.internal'),
        ('port', '5900')
) AS params(parameter_name, parameter_value)
WHERE connection_name = 'Cybertron (VNC)';

-- Grant guacadmin full access to both connections.
INSERT INTO guacamole_connection_permission (entity_id, connection_id, permission)
SELECT guacamole_entity.entity_id,
       guacamole_connection.connection_id,
       permission::guacamole_object_permission_type
FROM guacamole_entity
CROSS JOIN guacamole_connection
CROSS JOIN (
    VALUES ('READ'), ('UPDATE'), ('DELETE'), ('ADMINISTER')
) AS perms(permission)
WHERE guacamole_entity.name = 'guacadmin'
  AND guacamole_entity.type = 'USER'
  AND guacamole_connection.connection_name IN ('Cybertron (RDP)', 'Cybertron (VNC)');
