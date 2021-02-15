-- phpMyAdmin SQL Dump
-- version 5.0.4
-- https://www.phpmyadmin.net/
--
-- Vært: mariadb.mariadb.svc.cluster.local:3306
-- Genereringstid: 15. 02 2021 kl. 09:27:15
-- Serverversion: 10.5.8-MariaDB-1:10.5.8+maria~focal
-- PHP-version: 7.4.13

SET SQL_MODE = "NO_AUTO_VALUE_ON_ZERO";
START TRANSACTION;
SET time_zone = "+00:00";


/*!40101 SET @OLD_CHARACTER_SET_CLIENT=@@CHARACTER_SET_CLIENT */;
/*!40101 SET @OLD_CHARACTER_SET_RESULTS=@@CHARACTER_SET_RESULTS */;
/*!40101 SET @OLD_COLLATION_CONNECTION=@@COLLATION_CONNECTION */;
/*!40101 SET NAMES utf8mb4 */;

--
-- Database: `phoenix`
--

-- --------------------------------------------------------

--
-- Struktur-dump for tabellen `devices`
--

CREATE TABLE `devices` (
  `id` bigint(20) UNSIGNED NOT NULL,
  `guid` varchar(256) NOT NULL,
  `created` timestamp NOT NULL DEFAULT current_timestamp(),
  `token` varchar(256) DEFAULT NULL,
  `online` tinyint(4) NOT NULL DEFAULT 0
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- --------------------------------------------------------

--
-- Struktur-dump for tabellen `device_commands`
--

CREATE TABLE `device_commands` (
  `id` bigint(20) UNSIGNED NOT NULL,
  `device_id` bigint(20) UNSIGNED NOT NULL,
  `device_guid` varchar(256) NOT NULL,
  `command` varchar(256) NOT NULL,
  `created` timestamp NOT NULL DEFAULT current_timestamp(),
  `parameters` blob DEFAULT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- --------------------------------------------------------

--
-- Struktur-dump for tabellen `device_streams`
--

CREATE TABLE `device_streams` (
  `id` bigint(20) UNSIGNED NOT NULL,
  `device_id` bigint(20) UNSIGNED NOT NULL,
  `code` varchar(256) NOT NULL,
  `timestamp` timestamp NOT NULL DEFAULT current_timestamp() ON UPDATE current_timestamp(),
  `value` blob NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

--
-- Begrænsninger for dumpede tabeller
--

--
-- Indeks for tabel `devices`
--
ALTER TABLE `devices`
  ADD UNIQUE KEY `id` (`id`),
  ADD UNIQUE KEY `device_guid` (`guid`);

--
-- Indeks for tabel `device_commands`
--
ALTER TABLE `device_commands`
  ADD UNIQUE KEY `id` (`id`),
  ADD KEY `device_id` (`device_id`),
  ADD KEY `device_guid` (`device_guid`);

--
-- Indeks for tabel `device_streams`
--
ALTER TABLE `device_streams`
  ADD UNIQUE KEY `id` (`id`),
  ADD KEY `device_id` (`device_id`);

--
-- Brug ikke AUTO_INCREMENT for slettede tabeller
--

--
-- Tilføj AUTO_INCREMENT i tabel `devices`
--
ALTER TABLE `devices`
  MODIFY `id` bigint(20) UNSIGNED NOT NULL AUTO_INCREMENT;

--
-- Tilføj AUTO_INCREMENT i tabel `device_commands`
--
ALTER TABLE `device_commands`
  MODIFY `id` bigint(20) UNSIGNED NOT NULL AUTO_INCREMENT;

--
-- Tilføj AUTO_INCREMENT i tabel `device_streams`
--
ALTER TABLE `device_streams`
  MODIFY `id` bigint(20) UNSIGNED NOT NULL AUTO_INCREMENT;

--
-- Begrænsninger for dumpede tabeller
--

--
-- Begrænsninger for tabel `device_commands`
--
ALTER TABLE `device_commands`
  ADD CONSTRAINT `device_commands_ibfk_1` FOREIGN KEY (`device_id`) REFERENCES `devices` (`id`),
  ADD CONSTRAINT `device_commands_ibfk_2` FOREIGN KEY (`device_guid`) REFERENCES `devices` (`guid`);

--
-- Begrænsninger for tabel `device_streams`
--
ALTER TABLE `device_streams`
  ADD CONSTRAINT `device_streams_ibfk_1` FOREIGN KEY (`device_id`) REFERENCES `devices` (`id`);
COMMIT;

/*!40101 SET CHARACTER_SET_CLIENT=@OLD_CHARACTER_SET_CLIENT */;
/*!40101 SET CHARACTER_SET_RESULTS=@OLD_CHARACTER_SET_RESULTS */;
/*!40101 SET COLLATION_CONNECTION=@OLD_COLLATION_CONNECTION */;
