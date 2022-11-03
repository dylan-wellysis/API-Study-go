CREATE TABLE favorite (
  id int NOT NULL AUTO_INCREMENT,
  user_id int NOT NULL,
  article_id int NOT NULL,
  PRIMARY KEY (id),
  FOREIGN KEY (user_id) REFERENCES realworld_user (id),
  FOREIGN KEY (article_id) REFERENCES article (id)
)