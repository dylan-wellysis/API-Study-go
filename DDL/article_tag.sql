CREATE TABLE `article_tag` (
  `id` int NOT NULL AUTO_INCREMENT,
  `tag_id` int NOT NULL,
  `article_id` int NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `pair_uniq` (`tag_id`,`article_id`),
  KEY `article_id` (`article_id`),
  CONSTRAINT `article_tag_ibfk_1` FOREIGN KEY (`tag_id`) REFERENCES `tag` (`id`),
  CONSTRAINT `article_tag_ibfk_2` FOREIGN KEY (`article_id`) REFERENCES `article` (`id`)
)