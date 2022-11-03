CREATE TABLE `article` (
  `id` int NOT NULL AUTO_INCREMENT,
  `slug` varchar(100) NOT NULL,
  `title` varchar(100) NOT NULL,
  `description` text,
  `body` text,
  `tagList` text,
  `createdAt` datetime NOT NULL,
  `updatedAt` datetime NOT NULL,
  `favorited` tinyint(1) NOT NULL,
  `favoritesCount` int NOT NULL,
  `user_id` int NOT NULL,
  PRIMARY KEY (`id`),
  KEY `user_id` (`user_id`),
  CONSTRAINT `article_ibfk_1` FOREIGN KEY (`user_id`) REFERENCES `realworld_user` (`id`)
)