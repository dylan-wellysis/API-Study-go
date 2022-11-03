CREATE TABLE `realworld_user` (
  `id` int NOT NULL AUTO_INCREMENT,
  `username` varchar(30) NOT NULL,
  `bio` varchar(100) NOT NULL,
  `image` varchar(200) DEFAULT NULL,
  `following` tinyint(1) NOT NULL,
  PRIMARY KEY (`id`)
)