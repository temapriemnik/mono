from fastapi import FastAPI, HTTPException
from fastapi.middleware.cors import CORSMiddleware
import logging

from schemas.requests import ClassificationRequest, ClassificationResponse, EmbeddingRequest, EmbeddingResponse
from models.classifier import embedding_service
from config import settings

# Настройка логирования
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

app = FastAPI(
    title="Embedding & Intent Classification API",
    description="Унифицированный сервис для работы с эмбеддингами и классификации интентов",
    version="2.0.0"
)

# CORS middleware
app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

@app.post("/get_class", response_model=ClassificationResponse)
async def classify_intent(request: ClassificationRequest):
    """
    Классифицирует интент текста среди предложенных вариантов
    """
    try:
        if not request.text.strip():
            raise HTTPException(status_code=400, detail="Текст не может быть пустым")
        
        if not request.possible_intents:
            raise HTTPException(status_code=400, detail="Список возможных интентов не может быть пустым")
        
        if len(request.possible_intents) > settings.max_intents:
            raise HTTPException(
                status_code=400, 
                detail=f"Слишком много интентов. Максимум: {settings.max_intents}"
            )
        
        # Классифицируем интент
        intent, confidence, all_scores = embedding_service.classify_intent(
            text=request.text,
            possible_intents=request.possible_intents,
            examples=request.examples
        )
        
        # Если уверенность ниже порога, можно добавить логику для обработки
        if confidence < settings.similarity_threshold:
            logger.warning(f"Низкая уверенность классификации: {confidence}")
        
        return ClassificationResponse(
            intent=intent,
            confidence=confidence,
            all_scores=all_scores
        )
        
    except Exception as e:
        logger.error(f"Ошибка классификации: {str(e)}")
        raise HTTPException(status_code=500, detail=f"Ошибка классификации: {str(e)}")

@app.post("/embed", response_model=EmbeddingResponse)
async def get_embeddings(request: EmbeddingRequest):
    """
    Получает эмбеддинги для списка текстов
    """
    try:
        if not request.texts:
            raise HTTPException(status_code=400, detail="Список текстов не может быть пустым")
        
        if len(request.texts) > 100:  # Лимит на количество текстов
            raise HTTPException(status_code=400, detail="Слишком много текстов. Максимум: 100")
        
        # Получаем эмбеддинги
        embeddings = embedding_service.get_embeddings(
            texts=request.texts,
            normalize=request.normalize
        )
        
        return EmbeddingResponse(
            embeddings=embeddings,
            model=embedding_service.model_name,
            dimensions=embedding_service.dimensions
        )
        
    except Exception as e:
        logger.error(f"Ошибка получения эмбеддингов: {str(e)}")
        raise HTTPException(status_code=500, detail=f"Ошибка получения эмбеддингов: {str(e)}")

@app.get("/embed/info")
async def get_embedding_info():
    """Информация о модели эмбеддингов"""
    return {
        "model": embedding_service.model_name,
        "dimensions": embedding_service.dimensions,
        "max_sequence_length": embedding_service.model.max_seq_length
    }

@app.get("/health")
async def health_check():
    """Проверка здоровья сервиса"""
    return {
        "status": "healthy",
        "model": embedding_service.model_name,
        "service": "Embedding & Intent Classification API"
    }

@app.get("/")
async def root():
    return {
        "message": "Embedding & Intent Classification API", 
        "version": "2.0.0",
        "endpoints": {
            "classification": "/get_class",
            "embeddings": "/embed",
            "model_info": "/embed/info",
            "docs": "/docs"
        }
    }

if __name__ == "__main__":
    import uvicorn
    uvicorn.run(
        "main:app",
        host="localhost",
        port=8081,
        reload=True,
        log_level="info"
    )