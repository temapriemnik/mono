import torch
import numpy as np
from sentence_transformers import SentenceTransformer
from typing import List, Dict, Tuple, Union
import logging

logger = logging.getLogger(__name__)

class EmbeddingService:
    def __init__(self, model_name: str = "sentence-transformers/paraphrase-multilingual-MiniLM-L12-v2"):
        self.model = SentenceTransformer(model_name)
        self.model_name = model_name
        self.dimensions = self.model.get_sentence_embedding_dimension()
        logger.info(f"Загружена модель: {model_name}, размерность: {self.dimensions}")
    
    def encode_texts(self, texts: List[str], normalize: bool = True) -> np.ndarray:
        """Кодирует список текстов в эмбеддинги"""
        embeddings = self.model.encode(texts, convert_to_tensor=False)
        
        if normalize:
            # Нормализуем векторы
            embeddings = embeddings / np.linalg.norm(embeddings, axis=1, keepdims=True)
        
        return embeddings
    
    def get_embeddings(self, texts: List[str], normalize: bool = True) -> List[List[float]]:
        """Возвращает эмбеддинги в виде списка"""
        embeddings = self.encode_texts(texts, normalize)
        return embeddings.tolist()
    
    def calculate_similarity(self, text_embedding: np.ndarray, intent_embeddings: np.ndarray) -> np.ndarray:
        """Вычисляет косинусное сходство между текстом и интентами"""
        similarities = np.dot(intent_embeddings, text_embedding.T)
        return similarities.flatten()
    
    def classify_intent(
        self, 
        text: str, 
        possible_intents: List[str],
        examples: Dict[str, List[str]] = None
    ) -> Tuple[str, float, Dict[str, float]]:
        """
        Классифицирует интент текста среди возможных вариантов
        """
        
        # Если есть примеры, используем их для создания эмбеддингов интентов
        if examples:
            intent_texts = []
            intent_mapping = []
            
            for intent in possible_intents:
                if intent in examples and examples[intent]:
                    for example in examples[intent]:
                        intent_texts.append(example)
                        intent_mapping.append(intent)
                else:
                    # Если примеров нет, используем название интента как пример
                    intent_texts.append(intent)
                    intent_mapping.append(intent)
        else:
            # Используем названия интентов как тексты для сравнения
            intent_texts = possible_intents
            intent_mapping = possible_intents
        
        try:
            # Кодируем все тексты
            text_embedding = self.encode_texts([text])[0]  # Берем первый элемент
            intent_embeddings = self.encode_texts(intent_texts)
            
            # Вычисляем сходство
            similarities = self.calculate_similarity(text_embedding, intent_embeddings)
            
            # Агрегируем скоры по интентам (если несколько примеров на интент)
            intent_scores = {}
            for intent, score in zip(intent_mapping, similarities):
                if intent not in intent_scores:
                    intent_scores[intent] = []
                intent_scores[intent].append(float(score))
            
            # Берем максимальный скор для каждого интента
            max_scores = {intent: max(scores) for intent, scores in intent_scores.items()}
            
            # Находим лучший интент
            best_intent = max(max_scores, key=max_scores.get)
            best_score = max_scores[best_intent]
            
            return best_intent, best_score, max_scores
            
        except Exception as e:
            logger.error(f"Ошибка при классификации: {str(e)}")
            # Возвращаем дефолтный интент в случае ошибки
            return "help", 0.0, {intent: 0.0 for intent in possible_intents}

# Глобальный инстанс сервиса
embedding_service = EmbeddingService()