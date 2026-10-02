// Preview harness only: fake sessions so the Station can be judged without
// BurnMon running. Not shipped in burnmon-dev.exe.
(function(){
  var HUE = {'claude-code':'#ffa028','cowork':'#4da6ff','codex':'#a78bfa','copilot-cli':'#73d742','copilot-vscode':'#4ade80','hermes':'#e879f9'};
  function S(stage, sec, tool){ return [stage, sec, tool || '']; }
  var scripts = {
    a: {agent:'claude-code', project:'C:\\ZND\\50_projects\\burnmon', model:'claude-opus-5-5', tokens: 4.2e6, ctx: 0.41,
        loop:[S('reading',6,'Grep'),S('reading',5,'Read'),S('thinking',9),S('coding',8,'Edit'),S('running',10,'Bash'),S('thinking',6),S('delegating',30,'Task'),S('coding',7,'Write'),S('waiting',14)]},
    b: {agent:'codex', project:'C:\\ZND\\50_projects\\siteoffice', model:'gpt-6-sol', tokens: 9.1e6, ctx: 0.66,
        loop:[S('coding',9,'apply_patch'),S('running',12,'shell'),S('thinking',7),S('coding',6,'apply_patch'),S('running',8,'exec_command'),S('planning',6,'update_plan')]},
    c: {agent:'cowork', project:'C:\\ZND\\10_holding', model:'claude-opus-5-5', tokens: 2.3e6, ctx: 0.22,
        loop:[S('fetching',8,'WebSearch'),S('fetching',6,'mcp__slack__read_channel'),S('thinking',8),S('planning',7,'TaskCreate'),S('waiting',20)]},
    d: {agent:'copilot-cli', project:'C:\\ZND\\50_projects\\shoppilot', model:'gpt-6-luna', tokens: 0.6e6, ctx: 0.12,
        loop:[S('waiting',25),S('dormant',40)]},
    e: {agent:'hermes', project:'C:\\ZND\\50_projects\\cipher', model:'hermes-4', tokens: 1.1e6, ctx: null,
        loop:[S('dormant',60)]},
    f: {agent:'claude-code', project:'C:\\ZND\\50_projects\\dsi-engine', model:'claude-sonnet-5-5', tokens: 14.8e6, ctx: 0.93,
        loop:[S('coding',8,'MultiEdit'),S('compacting',8),S('reading',7,'Glob'),S('thinking',6),S('running',9,'Bash')]},
    g: {agent:'copilot-vscode', project:'C:\\ZND\\50_projects\\website', model:'gpt-6-sol', tokens: 0.9e6, ctx: 0.3,
        loop:[S('coding',10,'edit_file'),S('waiting',15),S('reading',6,'read_file')]},
    h: {agent:'codex', project:'C:\\ZND\\50_projects\\modelwatch', model:'gpt-6-sol', tokens: 3.3e6, ctx: 0.48,
        loop:[S('running',14,'shell'),S('thinking',8),S('fetching',6,'web_search'),S('waiting',12)]}
  };
  var start = Date.now(), live = ['a','b','c','d','e','f','g'], subOn = false, guest = false;
  function stageOf(sc, t){
    var total = sc.loop.reduce(function(n, s){ return n + s[1]; }, 0), x = (t + (sc.off || 0)) % total;
    for(var i = 0; i < sc.loop.length; i++){ if(x < sc.loop[i][1]) return {st: sc.loop[i], since: Date.now() - x * 1000}; x -= sc.loop[i][1]; }
    return {st: sc.loop[0], since: Date.now()};
  }
  Object.keys(scripts).forEach(function(k, i){ scripts[k].off = i * 7; });
  window.fakeModel = function(){
    var t = (Date.now() - start) / 1000, out = [];
    // a guest session docks every 70 s and leaves 45 s later
    var g = t % 70; guest = g > 20 && g < 65;
    var ids = live.concat(guest ? ['h'] : []);
    ids.forEach(function(k){
      var sc = scripts[k], r = stageOf(sc, t);
      var stage = r.st[0];
      if(k === 'h' && g < 35) stage = 'arriving';
      out.push({id: k, parent: null, agent: sc.agent, color: HUE[sc.agent], project: sc.project, model: sc.model,
                stage: stage, since: r.since, tool: r.st[2], tokens: sc.tokens + t * 900, ctx: sc.ctx});
      if(k === 'a' && stage === 'delegating'){
        ['a1','a2'].forEach(function(sid, j){
          var ss = [['reading','Read'],['reading','Grep'],['coding','Edit'],['running','Bash']][(Math.floor(t / 6) + j * 2) % 4];
          out.push({id: sid, parent: 'a', agent: 'claude-code', color: j ? '#ffc56b' : '#ffb347', project: sc.project + ' (explore)',
                    model: 'claude-haiku-4-5', stage: ss[0], since: Date.now() - 2000, tool: ss[1], tokens: 2e5 + t * 300, ctx: 0.1});
        });
      }
    });
    return {now: Date.now(), sessions: out};
  };
})();
