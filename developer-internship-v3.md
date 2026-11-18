

# **Internship: Developer Task** 

## **Problem Statement** 

Build a live polling tool. A user creates a poll, shares the link, and an audience votes on it. Everyone watching should see the results update live, as the votes come in, with no refresh needed. 

How it looks, what extra features you add, and where you draw the line is your call. Build something you'd genuinely be proud to show off. 

**Flow :** Create poll > Share link > Audience votes > Live results 

## **Tech Stack (required)** 

|**Layer**|**Use**|
|---|---|
|Frontend|React|
|Backend|Go (Gin)|
|Database|MongoDB|
|Realtime|Redis|



All four are required, and all four should be doing real work. Redis in particular should be driving the live updates/counts, not sitting there unused for show. 

## **Points to remember** 

- **Separation of concerns.** Keep frontend and backend code in their own folders. Don't mix concerns in the same file. 

- **Validate on the backend.** Never trust input straight from the client; check it server-side before it touches your database. 

- **Truly real-time.** If a user has to refresh the page to see a new vote, it doesn't count as real-time. 

- **Basic authentication.** Poll creation shouldn't be wide open to anyone with the URL. A simple login/signup is enough; we're not expecting anything elaborate, just a real check before someone can create or manage a poll. 

- **Actually deployed.** Ship it to a live, working link, not just `localhost` . 

## **Project Structure** 

No strict layout required, just keep it sensible. For example: 

```
/frontend     > React app
/backend      > Go service
README.md     > how to run it + your key decisions
```

As long as it's obvious where the frontend ends and the backend starts, you're free to organize it your way. 

## **Evaluation Criteria** 

- **Does it actually work.** The full flow runs end to end and results update live under real use, not just on a happy-path demo. 

- **Real use of the stack.** Redis and MongoDB are doing meaningful work, not bolted on to tick a box; the Go backend is solid, not just a passthrough. 

- **Code quality and security.** Clean, readable, sensibly structured code, backend input validation, and no obvious shortcuts. 

- **UI/UX, judgment, and polish.** How it looks and feels, the choices you made in the open parts of the brief, and whether it's properly live and deployed. 

- **Extra features.** Anything you add beyond the core flow, and whether it actually makes sense for the project, not just added for the sake of it. 

- **The live link.** Does it work, is it reachable, and does it hold up to actual use, not just look fine in a screenshot. 

- **The video.** Is it clear, honest, and does it actually answer what we asked, the challenge you faced and the AI question. 

## **Submission** 

Once you're done, email us: 

1. **GitHub repo** : public link. 

2. **Live deployed link** : working, publicly reachable. 

3. **A short video (3–5 min)** : unlisted YouTube or a public Google Drive link, where you walk through: The one challenge that gave you the most trouble, and how you solved it. 

   - Did you use any AI tools while building this? If yes, which ones and how did they help (or get in the way)? If no, just say so. 

Send everything to **<mark>devhiring@hclguvi.</mark> com** . 

**Note :** If any of the points is not followed, the internship application may be rejected. 

