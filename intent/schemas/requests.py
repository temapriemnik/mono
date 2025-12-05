from pydantic import BaseModel, Field
from typing import List, Optional, Union

class ClassificationRequest(BaseModel):
    text: str = Field(..., description="Текст для классификации")
    possible_intents: List[str] = Field(..., description="Список возможных интентов")
    examples: Optional[dict] = Field(None, description="Примеры для каждого интента")

class ClassificationResponse(BaseModel):
    intent: str = Field(..., description="Определенный интент")
    confidence: float = Field(..., description="Уверенность модели (0-1)")
    all_scores: Optional[dict] = Field(None, description="Скоры для всех интентов")

class EmbeddingRequest(BaseModel):
    texts: List[str] = Field(..., description="Список текстов для эмбеддинга")
    normalize: bool = Field(True, description="Нормализовать векторы")

class EmbeddingResponse(BaseModel):
    embeddings: List[List[float]] = Field(..., description="Векторы эмбеддингов")
    model: str = Field(..., description="Использованная модель")
    dimensions: int = Field(..., description="Размерность векторов")