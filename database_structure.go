package phoenix

var (
	DatabaseStructure = []string{
		"INVALID SQL, index 0 is not allowed for database updated",

		"CREATE TABLE `device_commands`(`id` bigint(20) UNSIGNED NOT NULL,`device_id` bigint(20) UNSIGNED NOT NULL,`device_guid` varchar(256) NOT NULL, `command` varchar(256) NOT NULL, `created` timestamp NOT NULL DEFAULT current_timestamp(), `parameters` blob DEFAULT NULL) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;",
		"ALTER TABLE `device_commands`  ADD UNIQUE KEY `id` (`id`), ADD KEY `device_id` (`device_id`), ADD KEY `device_guid` (`device_guid`);",
		"ALTER TABLE `device_commands` MODIFY `id` bigint(20) UNSIGNED NOT NULL AUTO_INCREMENT;",
		"ALTER TABLE `device_commands` ADD CONSTRAINT `device_commands_device_id_lock` FOREIGN KEY (`device_id`) REFERENCES `devices` (`id`)",
		"ALTER TABLE `devices` ADD UNIQUE KEY `device_guid` (`guid`);",
		"ALTER TABLE `device_commands` ADD CONSTRAINT `device_commands_device_guid_lock` FOREIGN KEY (`device_guid`) REFERENCES `devices` (`guid`);",
		"ALTER TABLE `device_commands` ADD `pending` TINYINT NOT NULL AFTER `parameters`;",
		"ALTER TABLE `devices` ADD `token_expiration` TIMESTAMP NULL AFTER `token`;",
	}
)
