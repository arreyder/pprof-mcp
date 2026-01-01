# Service Discovery Analysis

I need to understand what's happening with `{{SERVICE}}` in `{{ENV}}`. I've never looked at this service before and want to understand:

1. What is this service spending its time on?
2. Is there significant observability/infrastructure overhead?
3. Where is memory being allocated and at what rate?
4. Are there any obvious optimization opportunities?
5. Is there any contention or blocking I should know about?

Please download the latest profiles and give me a comprehensive analysis (you can use `pprof.discover`). Start broad and drill into anything interesting you find.
