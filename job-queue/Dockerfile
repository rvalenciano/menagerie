FROM python:3.12-slim
WORKDIR /src
COPY pyproject.toml ./
COPY src ./src
COPY demo.py ./
RUN pip install --no-cache-dir .
CMD ["python", "demo.py"]
