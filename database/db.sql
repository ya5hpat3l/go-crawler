-- Adminer 4.8.1 MySQL 8.0.30 dump

SET NAMES utf8;
SET time_zone = '+00:00';
SET foreign_key_checks = 0;

SET NAMES utf8mb4;

CREATE DATABASE `crawler` /*!40100 DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci */ /*!80016 DEFAULT ENCRYPTION='N' */;
USE `crawler`;

CREATE TABLE `pages` (
  `id` int NOT NULL AUTO_INCREMENT,
  `url` varchar(2048) NOT NULL,
  `last_crawl_time` bigint DEFAULT NULL,
  `crawl_count` int DEFAULT NULL,
  `file` varchar(1024) DEFAULT NULL,
  `new_pages` int DEFAULT NULL,
  `found_from` int NOT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=753941 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;


-- 2022-10-02 14:49:27

