#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
高性能 OpenAI API 密钥验证脚本
支持并发验证、去重、多模型测试
"""

import asyncio
import aiohttp
import time
import sys
import os
from typing import List, Dict, Tuple, Optional
from dataclasses import dataclass
import argparse

# 配置
DEFAULT_KEYS_FILE = "keys.txt"
DEFAULT_BASE_URL = "https://api.openai.com"
DEFAULT_CONCURRENCY = 50
DEFAULT_TIMEOUT = 30

# 测试模型列表
TEST_MODELS = [
    "gpt-4o-mini",
    "gpt-4.1-mini", 
    "gpt-4.1-nano"
]

@dataclass
class KeyValidationResult:
    """密钥验证结果"""
    key: str
    key_preview: str
    is_valid: bool
    model_results: Dict[str, bool]
    error_message: str = ""

class KeyValidator:
    """高性能密钥验证器"""
    
    def __init__(self, base_url: str = DEFAULT_BASE_URL, timeout: int = DEFAULT_TIMEOUT, concurrency: int = DEFAULT_CONCURRENCY):
        self.base_url = base_url.rstrip('/')
        self.timeout = timeout
        self.concurrency = concurrency
        self.session = None
        
    async def __aenter__(self):
        """异步上下文管理器入口"""
        connector = aiohttp.TCPConnector(
            limit=self.concurrency * 2,
            limit_per_host=self.concurrency,
            ttl_dns_cache=300,
            use_dns_cache=True,
        )
        
        timeout = aiohttp.ClientTimeout(total=self.timeout)
        self.session = aiohttp.ClientSession(
            connector=connector,
            timeout=timeout,
            headers={
                'User-Agent': 'GPT-Load-KeyValidator/1.0'
            }
        )
        return self
        
    async def __aexit__(self, exc_type, exc_val, exc_tb):
        """异步上下文管理器退出"""
        if self.session:
            await self.session.close()
    
    def get_key_preview(self, key: str) -> str:
        """获取密钥预览（脱敏显示）"""
        if len(key) < 20:
            return key[:4] + "***" + key[-4:]
        return key[:8] + "***" + key[-8:]
    
    async def test_model(self, key: str, model: str) -> bool:
        """测试单个模型是否可用"""
        url = f"{self.base_url}/v1/chat/completions"
        headers = {
            "Authorization": f"Bearer {key}",
            "Content-Type": "application/json"
        }
        
        payload = {
            "model": model,
            "messages": [
                {"role": "user", "content": "Hi"}
            ],
            "max_tokens": 1,
            "temperature": 0
        }
        
        try:
            async with self.session.post(url, headers=headers, json=payload) as response:
                if response.status == 200:
                    return True
                elif response.status == 401:
                    # 认证失败，密钥无效
                    return False
                elif response.status == 404:
                    # 模型不存在或无权限
                    return False
                elif response.status == 429:
                    # 速率限制，但密钥可能有效
                    return True
                else:
                    # 其他错误，认为模型不可用
                    return False
                    
        except asyncio.TimeoutError:
            return False
        except Exception:
            return False
    
    async def validate_key(self, key: str) -> KeyValidationResult:
        """验证单个密钥"""
        key_preview = self.get_key_preview(key)
        model_results = {}
        
        # 并发测试所有模型
        tasks = []
        for model in TEST_MODELS:
            task = asyncio.create_task(self.test_model(key, model))
            tasks.append((model, task))
        
        # 等待所有测试完成
        for model, task in tasks:
            try:
                result = await task
                model_results[model] = result
            except Exception:
                model_results[model] = False
        
        # 判断密钥是否有效（至少一个模型可用）
        is_valid = any(model_results.values())
        
        return KeyValidationResult(
            key=key,
            key_preview=key_preview,
            is_valid=is_valid,
            model_results=model_results
        )

def load_keys(file_path: str) -> List[str]:
    """加载密钥文件"""
    if not os.path.exists(file_path):
        print(f"❌ 密钥文件不存在: {file_path}")
        sys.exit(1)
    
    keys = []
    with open(file_path, 'r', encoding='utf-8') as f:
        for line_num, line in enumerate(f, 1):
            line = line.strip()
            if line and not line.startswith('#'):
                keys.append(line)
    
    return keys

def deduplicate_keys(keys: List[str]) -> Tuple[List[str], List[str]]:
    """去重密钥，返回(唯一密钥列表, 重复密钥列表)"""
    seen = set()
    unique_keys = []
    duplicate_keys = []

    for key in keys:
        if key not in seen:
            seen.add(key)
            unique_keys.append(key)
        else:
            duplicate_keys.append(key)

    return unique_keys, duplicate_keys

def format_model_status(model_results: Dict[str, bool]) -> str:
    """格式化模型状态显示"""
    status_parts = []
    for model in TEST_MODELS:
        if model in model_results:
            emoji = "✅" if model_results[model] else "❌"
            status_parts.append(f"{emoji} {model}")
        else:
            status_parts.append(f"❓ {model}")
    
    return " | ".join(status_parts)

async def validate_keys_batch(keys: List[str], base_url: str, timeout: int, concurrency: int) -> List[KeyValidationResult]:
    """批量验证密钥"""
    results = []
    
    async with KeyValidator(base_url, timeout, concurrency) as validator:
        # 创建信号量限制并发数
        semaphore = asyncio.Semaphore(concurrency)
        
        async def validate_with_semaphore(key: str) -> KeyValidationResult:
            async with semaphore:
                return await validator.validate_key(key)
        
        # 创建所有验证任务
        tasks = [validate_with_semaphore(key) for key in keys]
        
        # 使用 as_completed 来实时显示进度
        completed = 0
        total = len(tasks)
        
        print(f"\n🚀 开始验证 {total} 个密钥...")
        print("=" * 120)
        print(f"{'序号':<6} {'密钥预览':<20} {'状态':<6} {'模型测试结果':<80}")
        print("=" * 120)
        
        for coro in asyncio.as_completed(tasks):
            result = await coro
            completed += 1
            
            # 实时输出结果
            status_emoji = "✅ 有效" if result.is_valid else "❌ 无效"
            model_status = format_model_status(result.model_results)
            
            print(f"{completed:<6} {result.key_preview:<20} {status_emoji:<6} {model_status}")
            
            results.append(result)
    
    return results

def save_results(results: List[KeyValidationResult], duplicate_keys: Optional[List[str]] = None, output_dir: str = "."):
    """保存验证结果到文件"""
    valid_keys = []
    invalid_keys = []

    for result in results:
        if result.is_valid:
            valid_keys.append(result.key)
        else:
            invalid_keys.append(result.key)

    # 保存有效密钥
    valid_file = os.path.join(output_dir, "valid_keys.txt")
    with open(valid_file, 'w', encoding='utf-8') as f:
        for key in valid_keys:
            f.write(f"{key}\n")

    # 保存无效密钥
    invalid_file = os.path.join(output_dir, "invalid_keys.txt")
    with open(invalid_file, 'w', encoding='utf-8') as f:
        for key in invalid_keys:
            f.write(f"{key}\n")

    # 保存重复密钥
    duplicate_file = None
    if duplicate_keys:
        duplicate_file = os.path.join(output_dir, "duplicate_keys.txt")
        with open(duplicate_file, 'w', encoding='utf-8') as f:
            f.write("# 重复的密钥（已去重处理）\n")
            f.write(f"# 发现 {len(duplicate_keys)} 个重复密钥\n")
            f.write("# 这些密钥在验证过程中被自动去重\n\n")
            for key in duplicate_keys:
                f.write(f"{key}\n")

    return valid_file, invalid_file, duplicate_file, len(valid_keys), len(invalid_keys)

def print_summary(results: List[KeyValidationResult], valid_count: int, invalid_count: int,
                 valid_file: str, invalid_file: str, duplicate_file: Optional[str],
                 duplicate_count: int, duration: float):
    """打印验证总结"""
    total = len(results)

    print("\n" + "=" * 120)
    print("📊 验证结果总结")
    print("=" * 120)
    print(f"总密钥数量: {total}")
    print(f"有效密钥数: {valid_count} ({valid_count/total*100:.1f}%)")
    print(f"无效密钥数: {invalid_count} ({invalid_count/total*100:.1f}%)")
    if duplicate_count > 0:
        print(f"重复密钥数: {duplicate_count}")
    print(f"验证耗时: {duration:.2f} 秒")
    print(f"平均速度: {total/duration:.1f} 密钥/秒")
    print()
    print("📁 结果文件:")
    print(f"   有效密钥: {valid_file}")
    print(f"   无效密钥: {invalid_file}")
    if duplicate_file:
        print(f"   重复密钥: {duplicate_file}")

    # 模型统计
    print("\n📈 模型可用性统计:")
    model_stats = {model: 0 for model in TEST_MODELS}

    for result in results:
        if result.is_valid:
            for model, available in result.model_results.items():
                if available:
                    model_stats[model] += 1

    for model in TEST_MODELS:
        count = model_stats[model]
        percentage = count / valid_count * 100 if valid_count > 0 else 0
        print(f"   {model}: {count}/{valid_count} ({percentage:.1f}%)")

async def main():
    """主函数"""
    parser = argparse.ArgumentParser(description="OpenAI API 密钥验证工具")
    parser.add_argument("-f", "--file", default=DEFAULT_KEYS_FILE, help="密钥文件路径")
    parser.add_argument("-u", "--url", default=DEFAULT_BASE_URL, help="API 基础URL")
    parser.add_argument("-c", "--concurrency", type=int, default=DEFAULT_CONCURRENCY, help="并发数")
    parser.add_argument("-t", "--timeout", type=int, default=DEFAULT_TIMEOUT, help="超时时间(秒)")
    parser.add_argument("-o", "--output", default=".", help="输出目录")
    
    args = parser.parse_args()
    
    print("🔑 OpenAI API 密钥验证工具")
    print(f"📁 密钥文件: {args.file}")
    print(f"🌐 API地址: {args.url}")
    print(f"⚡ 并发数: {args.concurrency}")
    print(f"⏱️ 超时时间: {args.timeout}秒")
    print(f"🧪 测试模型: {', '.join(TEST_MODELS)}")
    
    # 加载和去重密钥
    print("\n📖 加载密钥文件...")
    raw_keys = load_keys(args.file)
    print(f"   原始密钥数量: {len(raw_keys)}")

    unique_keys, duplicate_keys = deduplicate_keys(raw_keys)
    duplicate_count = len(duplicate_keys)
    print(f"   去重后数量: {len(unique_keys)}")
    if duplicate_count > 0:
        print(f"   发现重复: {duplicate_count} 个")

    if not unique_keys:
        print("❌ 没有找到有效的密钥")
        sys.exit(1)

    # 开始验证
    start_time = time.time()
    results = await validate_keys_batch(unique_keys, args.url, args.timeout, args.concurrency)
    duration = time.time() - start_time

    # 保存结果
    valid_file, invalid_file, duplicate_file, valid_count, invalid_count = save_results(
        results, duplicate_keys if duplicate_keys else None, args.output
    )

    # 打印总结
    print_summary(results, valid_count, invalid_count, valid_file, invalid_file,
                 duplicate_file, duplicate_count, duration)

if __name__ == "__main__":
    try:
        asyncio.run(main())
    except KeyboardInterrupt:
        print("\n\n⚠️ 用户中断验证过程")
        sys.exit(1)
    except Exception as e:
        print(f"\n❌ 验证过程中发生错误: {e}")
        sys.exit(1)
