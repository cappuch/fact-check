import discord
from discord.ext import commands
from discord import app_commands
import os
from dotenv import load_dotenv
import asyncio

load_dotenv()

from fact_checker import perform_fact_check
from db import init_db, add_fact_check
import json

init_db()

class FactCheckBot(discord.Client):
    def __init__(self):
        super().__init__(intents=discord.Intents.default())
        self.tree = app_commands.CommandTree(self)

    async def setup_hook(self):
        await self.tree.sync()

client = FactCheckBot()

@client.event
async def on_ready():
    print(f'Logged in as {client.user} (ID: {client.user.id})')

@client.tree.command(name="factcheck", description="Fact check a statement")
@app_commands.describe(query="The statement or claim you want to fact check")
async def factcheck(interaction: discord.Interaction, query: str):
    await interaction.response.defer(thinking=True)
    
    result = await perform_fact_check(query)
    summary = result.get('summary', 'Failed to retrieve summary.')
    sources_json = result.get('sources', '[]')
    
    try:
        sources_list = json.loads(sources_json)
    except Exception:
        sources_list = []
    
    add_fact_check(query, summary, sources_json)
    
    embed = discord.Embed(
        title=f"Fact Check: {query}",
        description=summary,
        color=discord.Color.blue()
    )
    
    if sources_list:
        sources_text = ""
        for i, src in enumerate(sources_list[:5], 1):
            sources_text += f"{i}. [{src['title']}]({src['url']})\n"
        embed.add_field(name="Sources", value=sources_text, inline=False)
    
    await interaction.followup.send(embed=embed)

if __name__ == "__main__":
    token = os.environ.get("DISCORD_TOKEN")
    if not token:
        print("Error: DISCORD_TOKEN is missing from your environment.")
    else:
        client.run(token)
