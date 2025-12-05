from pydantic_settings import BaseSettings

class Settings(BaseSettings):
    app_name: str = "Intent Classification API"
    model_name: str = "sentence-transformers/paraphrase-multilingual-MiniLM-L12-v2"
    similarity_threshold: float = 0.3
    max_intents: int = 10
    
    class Config:
        env_file = ".env"

settings = Settings()