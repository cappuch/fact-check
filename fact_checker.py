import os
from exa_py import Exa
from openai import AsyncOpenAI
import json

async def perform_fact_check(query: str):
    print(f"QUERY | {query}")
    exa_api_key = os.environ.get("EXA_API_KEY")
    openai_api_key = os.environ.get("OPENAI_API_KEY")
    openai_base_url = os.environ.get("OPENAI_BASE_URL")
    openai_model = os.environ.get("OPENAI_MODEL_ID")

    if not exa_api_key or not openai_api_key:
        return {
            "summary": "API keys for Exa or OpenAI are missing in the environment. Please check your .env file.",
            "sources": "[]"
        }

    exa = Exa(api_key=exa_api_key)
    openai_client = AsyncOpenAI(api_key=openai_api_key, base_url=openai_base_url)

    try:
        results = exa.search_and_contents(
            query,
            type="deep", # deep research mode
            num_results=10,
            highlights={"max_characters": 4000}
        )
    except Exception as e:
        return {
            "summary": f"Failed to perform search with Exa API: {str(e)}",
            "sources": "[]"
        }

    if not results.results:
        return {
            "summary": "Could not find any relevant information to fact-check this query.",
            "sources": "[]"
        }

    sources_data = []
    context_text = ""
    for i, res in enumerate(results.results, 1):
        highlights = "\\n".join(res.highlights) if res.highlights else "No highlights available."
        context_text += f"Source {i}: {res.title} (URL: {res.url})\\nHighlights:\\n{highlights}\\n\\n"
        sources_data.append({
            "title": res.title,
            "url": res.url
        })
    
    system_prompt = (
        "You are a professional fact-checking assistant. Your job is to analyze the provided search "
        "results and determine the accuracy of the user's query. Provide a clear, unbiased summary "
        "explaining whether the claim is true, false, mixed, or unverifiable. Base your analysis completely "
        "on the provided context, and cite the sources when making your points (e.g., 'According to Source 1...')."
    )
    
    user_prompt = f"Query to fact-check: {query}\\n\\nContext:\\n{context_text}"

    try:
        completion = await openai_client.chat.completions.create(
            model=openai_model,
            messages=[
                {"role": "system", "content": system_prompt},
                {"role": "user", "content": user_prompt}
            ],
            temperature=0.3
        )
        summary = completion.choices[0].message.content
        
    except Exception as e:
        print(f"ERROR! | {e}")
        return {
            "summary": f"Failed to generate summary.",
            "sources": json.dumps(sources_data)
        }

    return {
        "summary": summary,
        "sources": json.dumps(sources_data)
    }
