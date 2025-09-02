/*
Copyright (c) 2025 Tethys Plex

This file is part of Veloera.

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.
*/

import { useState, useCallback } from 'react';
import { ModelMappingUtils } from '../utils/modelMappingUtils';

/**
 * 模型映射管理 Hook
 * 提供模型映射相关的状态管理和操作方法
 */
export const useModelMapping = (initialMapping = {}) => {
  // 用于追踪模型的原始名称映射关系 { displayName: originalName }
  const [modelOriginalMapping, setModelOriginalMapping] = useState(initialMapping);

  /**
   * 更新模型列表和映射关系
   * @param {Array} newModels - 新的模型列表
   * @param {Object} newMapping - 新的映射关系
   * @param {Function} updateModelsCallback - 更新模型列表的回调函数
   */
  const updateModelsList = useCallback((newModels, newMapping, updateModelsCallback) => {
    const uniqueModels = ModelMappingUtils.filterAndDedupeModels(newModels);
    
    if (updateModelsCallback) {
      updateModelsCallback(uniqueModels);
    }
    
    setModelOriginalMapping(newMapping);
  }, []);

  /**
   * 同步模型映射到模型配置
   * @param {string} mappingValue - JSON 格式的映射配置
   * @param {Array} currentModels - 当前模型列表
   * @param {Function} updateModelsCallback - 更新模型列表的回调函数
   */
  const syncModelMappingToModels = useCallback((mappingValue, currentModels, updateModelsCallback) => {
    const mapping = ModelMappingUtils.parseModelMapping(mappingValue);

    if (!mapping || Object.keys(mapping).length === 0) {
      // 当映射为空时，恢复为原始名称
      if (!currentModels || currentModels.length === 0) {
        return;
      }

      // 没有映射关系，则不需要恢复
      if (!modelOriginalMapping || Object.keys(modelOriginalMapping).length === 0) {
        updateModelsList(currentModels, {}, updateModelsCallback);
        return;
      }

      // 恢复模型到原始名称
      const restoredModels = ModelMappingUtils.restoreModelsToOriginal(
        currentModels, 
        modelOriginalMapping
      );

      // 清空原始映射关系，因为已经恢复完成
      updateModelsList(restoredModels, {}, updateModelsCallback);
      return;
    }

    // 在应用新映射之前，先恢复当前模型到原始名称
    const restoredModels = ModelMappingUtils.restoreModelsToOriginal(
      currentModels, 
      modelOriginalMapping
    );

    const { updatedModels, newMapping, hasChanges } = ModelMappingUtils.applyModelMapping(
      mapping,
      restoredModels,
      {}
    );

    if (hasChanges || !ModelMappingUtils.areModelsEqual(currentModels, updatedModels)) {
      updateModelsList(updatedModels, newMapping, updateModelsCallback);
    }
  }, [modelOriginalMapping, updateModelsList]);

  /**
   * 初始化模型映射关系
   * @param {string} mappingValue - JSON 格式的映射配置
   * @param {Array} models - 模型列表
   */
  const initializeModelMapping = useCallback((mappingValue, models) => {
    const mapping = ModelMappingUtils.parseModelMapping(mappingValue);
    
    if (mapping && Object.keys(mapping).length > 0) {
    const initialMapping = Object.create(null);
    const DANGEROUS_KEYS = new Set(['__proto__', 'prototype', 'constructor']);  
      
      // 根据当前的模型映射和模型列表，建立原始映射关系
      Object.entries(mapping).forEach(([displayName, originalName]) => {
        const displayNameTrimmed = displayName.trim();
        const originalNameTrimmed = typeof originalName === 'string' ? originalName.trim() : null;
        
        if (displayNameTrimmed && originalNameTrimmed && !DANGEROUS_KEYS.has(displayNameTrimmed) && models.includes(displayNameTrimmed)) {
          initialMapping[displayNameTrimmed] = originalNameTrimmed;
        }
      });
      
      setModelOriginalMapping(initialMapping);
    } else {
      setModelOriginalMapping({});
    }
  }, []);

  /**
   * 重置映射关系
   */
  const resetModelMapping = useCallback(() => {
    setModelOriginalMapping({});
  }, []);

  return {
    modelOriginalMapping,
    updateModelsList,
    syncModelMappingToModels,
    initializeModelMapping,
    resetModelMapping,
    setModelOriginalMapping
  };
};