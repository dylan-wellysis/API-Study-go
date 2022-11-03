CREATE TABLE `tag` (
  `id` int NOT NULL AUTO_INCREMENT,
  `tag_name` varchar(20) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `tag_name` (`tag_name`)
)