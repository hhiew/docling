# 采集任务配置

定时采集任务按分钟粒度下发，支持断点续传。

## 参数说明

- interval：采集间隔，默认 60 秒
- timeout：单次请求超时
- retry：失败重试次数

## 下发示例

```bash
curl -X POST /api/v1/tasks -d '{"interval": 60}'
```
