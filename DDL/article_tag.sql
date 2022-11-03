CREATE TABLE article_tag (
  id int NOT NULL AUTO_INCREMENT,
  tag_id int NOT NULL,
  article_id int NOT NULL,
  PRIMARY KEY (id),
  FOREIGN KEY (tag_id) REFERENCES tag (id),
  FOREIGN KEY (article_id) REFERENCES article (id)
)