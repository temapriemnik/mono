import os
import logging
from logging.handlers import RotatingFileHandler

# Импортируем ТОЛЬКО нужные загрузчики (без unstructured!)
from langchain_community.document_loaders import (
    PyPDFLoader,
    TextLoader,
    BSHTMLLoader,      # ← заменяет UnstructuredHTMLLoader
    Docx2txtLoader,    # ← можно импортировать здесь, а не внутри функции
)

from langchain.text_splitter import RecursiveCharacterTextSplitter
from langchain_mistralai import MistralAIEmbeddings
from langchain_chroma import Chroma

# Настройка логгера
LOG_DIR = "/app/logs"
os.makedirs(LOG_DIR, exist_ok=True)

logger = logging.getLogger("rag_ingest")
logger.setLevel(logging.INFO)

formatter = logging.Formatter("%(asctime)s — %(levelname)s — %(message)s")
file_handler = RotatingFileHandler(
    os.path.join(LOG_DIR, "app.log"), maxBytes=10 * 1024 * 1024, backupCount=5
)
file_handler.setFormatter(formatter)
logger.addHandler(file_handler)

CHROMA_PATH = "/app/chroma"
DOCUMENTS_PATH = "/app/documents"


def load_single_document(file_path):
    ext = os.path.splitext(file_path)[1].lower()
    try:
        if ext == ".pdf":
            return PyPDFLoader(file_path).load()
        elif ext == ".txt":
            return TextLoader(file_path, encoding="utf-8").load()
        elif ext in [".html", ".htm"]:
            return BSHTMLLoader(file_path, open_encoding="utf-8").load()
        elif ext == ".docx":
            return Docx2txtLoader(file_path).load()
        else:
            logger.warning(f"Пропущен неподдерживаемый файл: {file_path}")
            return []
    except Exception as e:
        logger.error(f"Ошибка загрузки {file_path}: {e}", exc_info=True)
        return []


def ingest_documents():
    logger.info("Начало индексации...")
    docs = []
    for root, _, files in os.walk(DOCUMENTS_PATH):
        for file in files:
            full_path = os.path.join(root, file)
            docs.extend(load_single_document(full_path))

    if not docs:
        logger.warning("Нет документов для индексации")
        return

    text_splitter = RecursiveCharacterTextSplitter(chunk_size=800, chunk_overlap=100)
    splits = text_splitter.split_documents(docs)

    logger.info(f"Создание эмбеддингов для {len(splits)} чанков...")
    embeddings = MistralAIEmbeddings(model="mistral-embed")
    Chroma.from_documents(
        documents=splits,
        embedding=embeddings,
        persist_directory=CHROMA_PATH,
    )
    logger.info("Индексация завершена")